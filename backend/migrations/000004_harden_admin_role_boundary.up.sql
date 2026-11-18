-- Harden the membership database boundary.
--
-- 000003 introduced the role based write path, but it left two gaps:
--
--   * The tables could still be owned by the role that applied the migrations.
--     A table owner can disable row level security, drop policies or grant
--     itself write privileges, so in deployments where the API credential ran
--     the migrations the write boundary was decorative.
--   * The guard inside the admin functions checked the same role that holds
--     the EXECUTE grant. Granting that group to another role therefore opened
--     the door, contrary to what the documentation claimed.
--
-- This migration transfers the ownership to energyhub_owner, removes every
-- leftover privilege, checks an independent capability inside the functions,
-- removes the read bypass of the admin group, records an audit trail for every
-- administrative write and turns a repeated organization id into an explicit
-- conflict instead of a silent rename.
--
-- Prerequisite: backend/deploy/postgres/bootstrap.sql must have been applied.
-- The capability role is a cluster role and cannot be created by the
-- migration itself.

BEGIN;

-- Ownership -----------------------------------------------------------------------

-- energyhub_owner needs CREATE on public to own the tables, and it owns the
-- app schema so it can own the functions and the audit trail. The bootstrap
-- grants it; the migration only does it when the migrator may grant it.
DO $$
BEGIN
    IF has_schema_privilege('energyhub_owner', 'public', 'CREATE') THEN
        RETURN;
    END IF;
    IF (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)
       OR pg_has_role(current_user, 'pg_database_owner', 'member') THEN
        EXECUTE 'GRANT CREATE ON SCHEMA public TO energyhub_owner';
    ELSE
        RAISE EXCEPTION 'energyhub_owner needs CREATE on schema public: run the bootstrap as a superuser';
    END IF;
END
$$;

ALTER SCHEMA app OWNER TO energyhub_owner;

ALTER TABLE public.organizations OWNER TO energyhub_owner;
ALTER TABLE public.organization_memberships OWNER TO energyhub_owner;

-- Privileges ----------------------------------------------------------------------

-- Every privilege that existed before is revoked first. A leftover grant would
-- re-open a write path that this migration closes.
REVOKE ALL ON public.organizations, public.organization_memberships FROM PUBLIC;
REVOKE ALL ON public.organizations, public.organization_memberships
    FROM energyhub_runtime, energyhub_admin, energyhub_membership_admin;

GRANT USAGE ON SCHEMA public TO energyhub_runtime;
GRANT SELECT ON public.organizations, public.organization_memberships TO energyhub_runtime;

-- No application role may create objects in public: that would allow shadow
-- tables or functions next to the protected ones.
REVOKE CREATE ON SCHEMA public FROM energyhub_runtime, energyhub_admin, energyhub_membership_admin;

-- The administrative credential executes the admin functions and nothing else.
-- It does not need to read the tenant tables of every organization.
REVOKE SELECT ON public.organizations, public.organization_memberships FROM energyhub_membership_admin;

-- Audit trail ----------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS app.admin_audit_log (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor TEXT NOT NULL,
    operation TEXT NOT NULL CHECK (operation IN ('create_organization', 'grant_membership', 'revoke_membership')),
    organization_id TEXT NOT NULL,
    subject TEXT,
    role TEXT
);

ALTER TABLE app.admin_audit_log OWNER TO energyhub_owner;

REVOKE ALL ON app.admin_audit_log FROM PUBLIC;
REVOKE ALL ON app.admin_audit_log FROM energyhub_runtime, energyhub_admin, energyhub_membership_admin;
GRANT SELECT ON app.admin_audit_log TO energyhub_membership_admin;

-- Administrative functions ---------------------------------------------------------

-- The guard now checks energyhub_admin_capability, a role that only the admin
-- credential holds, while the EXECUTE grant stays on energyhub_membership_admin.
-- Granting one of them by mistake is therefore not enough to write.
CREATE OR REPLACE FUNCTION app.create_organization(p_organization_id text, p_name text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_admin_capability', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_name IS NULL OR btrim(p_name) = '' THEN
        RAISE EXCEPTION 'organization name is required' USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.organizations (id, name)
    VALUES (btrim(p_organization_id), btrim(p_name));

    INSERT INTO app.admin_audit_log (actor, operation, organization_id)
    VALUES (session_user, 'create_organization', btrim(p_organization_id));
EXCEPTION
    WHEN unique_violation THEN
        RAISE EXCEPTION 'organization % already exists', btrim(p_organization_id) USING ERRCODE = '23505';
END;
$$;

CREATE OR REPLACE FUNCTION app.grant_membership(p_organization_id text, p_subject text, p_role text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_admin_capability', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_subject IS NULL OR btrim(p_subject) = '' THEN
        RAISE EXCEPTION 'subject is required' USING ERRCODE = '22023';
    END IF;
    IF p_role IS NULL OR p_role NOT IN ('owner', 'admin', 'member', 'viewer') THEN
        RAISE EXCEPTION 'invalid membership role' USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.organization_memberships (organization_id, subject, role)
    VALUES (btrim(p_organization_id), btrim(p_subject), p_role)
    ON CONFLICT (organization_id, subject) DO UPDATE SET role = EXCLUDED.role;

    INSERT INTO app.admin_audit_log (actor, operation, organization_id, subject, role)
    VALUES (session_user, 'grant_membership', btrim(p_organization_id), btrim(p_subject), p_role);
END;
$$;

CREATE OR REPLACE FUNCTION app.revoke_membership(p_organization_id text, p_subject text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_admin_capability', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_subject IS NULL OR btrim(p_subject) = '' THEN
        RAISE EXCEPTION 'subject is required' USING ERRCODE = '22023';
    END IF;

    DELETE FROM public.organization_memberships
    WHERE organization_id = btrim(p_organization_id) AND subject = btrim(p_subject);

    INSERT INTO app.admin_audit_log (actor, operation, organization_id, subject)
    VALUES (session_user, 'revoke_membership', btrim(p_organization_id), btrim(p_subject));
END;
$$;

-- Policies -----------------------------------------------------------------------

-- The select policies no longer let the admin group read every tenant. The
-- administrative credential writes through the functions; it does not read the
-- membership tables.
DROP POLICY IF EXISTS organizations_select_member ON public.organizations;
CREATE POLICY organizations_select_member ON public.organizations
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1
            FROM public.organization_memberships AS membership
            WHERE membership.organization_id = public.organizations.id
              AND membership.subject = current_setting('app.current_subject', true)
        )
    );

DROP POLICY IF EXISTS memberships_select_own ON public.organization_memberships;
CREATE POLICY memberships_select_own ON public.organization_memberships
    FOR SELECT
    USING (
        subject = current_setting('app.current_subject', true)
    );

-- Leftover write privileges -------------------------------------------------------

-- Any login role that could still write the tenant tables loses that
-- privilege. A table owner is already covered by the ownership transfer above.
DO $$
DECLARE
    role_name text;
BEGIN
    FOR role_name IN
        SELECT rolname FROM pg_roles WHERE rolcanlogin AND NOT rolsuper
    LOOP
        IF has_table_privilege(role_name, 'public.organizations', 'INSERT')
           OR has_table_privilege(role_name, 'public.organizations', 'UPDATE')
           OR has_table_privilege(role_name, 'public.organizations', 'DELETE')
           OR has_table_privilege(role_name, 'public.organization_memberships', 'INSERT')
           OR has_table_privilege(role_name, 'public.organization_memberships', 'UPDATE')
           OR has_table_privilege(role_name, 'public.organization_memberships', 'DELETE') THEN
            EXECUTE format(
                'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.organizations, public.organization_memberships FROM %I',
                role_name);
        END IF;
    END LOOP;
END
$$;

-- Fail closed if a login role can still write the tenant tables. The roles that
-- inherit energyhub_owner are excluded on purpose: that is the migration
-- credential, which is privileged by design and must never be the credential
-- of the API.
DO $$
DECLARE
    offending text;
BEGIN
    SELECT string_agg(DISTINCT role_name.rolname, ', ' ORDER BY role_name.rolname)
      INTO offending
      FROM pg_roles AS role_name
     WHERE role_name.rolcanlogin
       AND NOT role_name.rolsuper
       AND NOT pg_has_role(role_name.rolname, 'energyhub_owner', 'member')
       AND (has_table_privilege(role_name.rolname, 'public.organizations', 'INSERT')
            OR has_table_privilege(role_name.rolname, 'public.organizations', 'UPDATE')
            OR has_table_privilege(role_name.rolname, 'public.organizations', 'DELETE')
            OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'INSERT')
            OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'UPDATE')
            OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'DELETE'));
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'login roles still have write privileges on the tenant tables: %', offending;
    END IF;
END
$$;

COMMIT;
