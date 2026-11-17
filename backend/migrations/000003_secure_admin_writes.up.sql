-- Replace the forgeable app.is_system_admin write boundary with a database role
-- boundary plus audited SECURITY DEFINER functions.
--
-- A custom GUC cannot be protected: every role can run
-- SET app.is_system_admin = 'on', so the previous write policy could be
-- satisfied by any statement that ran with the application credential.
-- Writes now require the energyhub_membership_admin role, and the only
-- supported path is the app schema functions below.
--
-- Roles expected to exist before this migration runs (see
-- backend/deploy/postgres/bootstrap.sql):
--   energyhub_runtime        LOGIN, read-only credential used by the API
--   energyhub_membership_admin NOLOGIN group that grants administrative writes
--   energyhub_admin          LOGIN, member of energyhub_membership_admin
--   energyhub_owner          NOLOGIN BYPASSRLS, owner of the admin functions

CREATE SCHEMA IF NOT EXISTS app;
REVOKE CREATE ON SCHEMA app FROM PUBLIC;

-- Administrative write functions -------------------------------------------------

CREATE OR REPLACE FUNCTION app.create_organization(p_organization_id text, p_name text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_membership_admin', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_name IS NULL OR btrim(p_name) = '' THEN
        RAISE EXCEPTION 'organization name is required' USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.organizations (id, name)
    VALUES (btrim(p_organization_id), btrim(p_name))
    ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
END;
$$;

CREATE OR REPLACE FUNCTION app.grant_membership(p_organization_id text, p_subject text, p_role text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_membership_admin', 'member') THEN
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
END;
$$;

CREATE OR REPLACE FUNCTION app.revoke_membership(p_organization_id text, p_subject text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_membership_admin', 'member') THEN
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
END;
$$;

-- Function ownership and execution ---------------------------------------------

ALTER FUNCTION app.create_organization(text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.grant_membership(text, text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.revoke_membership(text, text) OWNER TO energyhub_owner;

REVOKE ALL ON FUNCTION app.create_organization(text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION app.grant_membership(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION app.revoke_membership(text, text) FROM PUBLIC;

GRANT USAGE ON SCHEMA app TO energyhub_membership_admin;
GRANT EXECUTE ON FUNCTION app.create_organization(text, text) TO energyhub_membership_admin;
GRANT EXECUTE ON FUNCTION app.grant_membership(text, text, text) TO energyhub_membership_admin;
GRANT EXECUTE ON FUNCTION app.revoke_membership(text, text) TO energyhub_membership_admin;

-- Table privileges --------------------------------------------------------------

REVOKE ALL ON public.organizations, public.organization_memberships FROM PUBLIC;

GRANT USAGE ON SCHEMA public TO energyhub_runtime, energyhub_membership_admin;
GRANT SELECT ON public.organizations, public.organization_memberships TO energyhub_runtime;
GRANT SELECT ON public.organizations, public.organization_memberships TO energyhub_membership_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.organizations, public.organization_memberships TO energyhub_owner;

-- Policies ----------------------------------------------------------------------

DROP POLICY IF EXISTS organizations_write_admin ON organizations;
DROP POLICY IF EXISTS memberships_write_admin ON organization_memberships;

DROP POLICY IF EXISTS organizations_select_member ON organizations;
CREATE POLICY organizations_select_member ON organizations
    FOR SELECT
    USING (
        pg_has_role(current_user, 'energyhub_membership_admin', 'member')
        OR EXISTS (
            SELECT 1
            FROM organization_memberships AS membership
            WHERE membership.organization_id = organizations.id
              AND membership.subject = current_setting('app.current_subject', true)
        )
    );

DROP POLICY IF EXISTS memberships_select_own ON organization_memberships;
CREATE POLICY memberships_select_own ON organization_memberships
    FOR SELECT
    USING (
        subject = current_setting('app.current_subject', true)
        OR pg_has_role(current_user, 'energyhub_membership_admin', 'member')
    );
