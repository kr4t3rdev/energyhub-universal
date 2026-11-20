-- Tenant isolation by PostgreSQL role per organization.
--
-- Replaces the forgeable app.current_subject GUC for reads of tenant data with
-- a database role boundary: for each organization there is a dedicated
-- energyhub_tenant_<id> role, and the API credential (energyhub_runtime) can
-- only read that organization's data after assuming its role with SET ROLE.
--
-- The mapping from organization_id to role name is deterministic:
--   energyhub_tenant_<organization_id with non-alphanumerics replaced by _>
-- Examples:
--   org-1          -> energyhub_tenant_org_1
--   11111111-...   -> energyhub_tenant_11111111111141118111111111111111
--
-- The mapping is applied automatically when an organization is created
-- (see app.create_organization) and backfilled for existing organizations
-- by this migration.
--
-- energyhub_runtime is a member of every tenant role, so it can SET ROLE to
-- the role of the organization it wants to read. The application verifies
-- membership before assuming a tenant role, and performs SET ROLE directly
-- (not through a security-definer function, since SET ROLE is not permitted
-- inside SECURITY DEFINER).
--

BEGIN;

-- ---------------------------------------------------------------------------
-- Helper: deterministic role name for an organization
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION app.tenant_role_name(p_organization_id text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
SET search_path = ''
AS $$
DECLARE
    sanitized text;
BEGIN
    -- Replace anything that is not a letter, digit or underscore with _.
    sanitized := regexp_replace(btrim(p_organization_id), '[^a-zA-Z0-9_]', '_', 'g');
    -- PostgreSQL role names cannot start with a digit.
    IF sanitized ~ '^[0-9]' THEN
        sanitized := 'tenant_' || sanitized;
    END IF;
    RETURN 'energyhub_tenant_' || sanitized;
END;
$$;

ALTER FUNCTION app.tenant_role_name(text) OWNER TO energyhub_owner;
REVOKE ALL ON FUNCTION app.tenant_role_name(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.tenant_role_name(text) TO energyhub_runtime, energyhub_membership_admin;

-- ---------------------------------------------------------------------------
-- Backfill tenant roles for existing organizations
-- ---------------------------------------------------------------------------

DO $$
DECLARE
    org_record record;
    role_name text;
BEGIN
    FOR org_record IN
        SELECT id FROM public.organizations
    LOOP
        role_name := app.tenant_role_name(org_record.id);

        -- Create the tenant role if it does not exist.
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS', role_name);
        END IF;

        -- energyhub_owner is a member so app.set_tenant_role can SET ROLE.
        EXECUTE format('GRANT %I TO energyhub_owner', role_name);

        -- energyhub_runtime is a member so the API can SET ROLE.
        EXECUTE format('GRANT %I TO energyhub_runtime', role_name);

        -- The tenant role can read its own organization's rows.
        EXECUTE format('GRANT USAGE ON SCHEMA public TO %I', role_name);
        EXECUTE format('GRANT USAGE ON SCHEMA app TO %I', role_name);
        EXECUTE format('GRANT SELECT ON public.organizations TO %I', role_name);
        EXECUTE format('GRANT SELECT ON public.organization_memberships TO %I', role_name);
        EXECUTE format('GRANT SELECT ON public.sites TO %I', role_name);
        EXECUTE format('GRANT SELECT ON public.site_devices TO %I', role_name);
    END LOOP;
END
$$;

-- ---------------------------------------------------------------------------
-- create_organization creates the tenant role automatically
--
-- When a new organization is created, its tenant role is created at the same
-- time so the API can SET ROLE to it later. energyhub_owner and
-- energyhub_runtime are made members so the SET ROLE gate and the API both
-- work.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION app.create_organization(p_organization_id text, p_name text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
DECLARE
    role_name text;
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

    -- Create the tenant role for this organization.
    role_name := app.tenant_role_name(btrim(p_organization_id));

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
        EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS', role_name);
    END IF;

    EXECUTE format('GRANT %I TO energyhub_owner', role_name);
    EXECUTE format('GRANT %I TO energyhub_runtime', role_name);
    EXECUTE format('GRANT SELECT ON public.organizations TO %I', role_name);
    EXECUTE format('GRANT SELECT ON public.organization_memberships TO %I', role_name);
    EXECUTE format('GRANT USAGE ON SCHEMA public TO %I', role_name);
    EXECUTE format('GRANT USAGE ON SCHEMA app TO %I', role_name);
    EXECUTE format('GRANT SELECT ON public.sites TO %I', role_name);
    EXECUTE format('GRANT SELECT ON public.site_devices TO %I', role_name);
EXCEPTION
    WHEN unique_violation THEN
        RAISE EXCEPTION 'organization already exists';
END;
$$;

ALTER FUNCTION app.create_organization(text, text) OWNER TO energyhub_owner;
REVOKE ALL ON FUNCTION app.create_organization(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.create_organization(text, text) TO energyhub_membership_admin;

-- ---------------------------------------------------------------------------
-- Fail closed: no login role other than energyhub_owner may write tenant tables
-- ---------------------------------------------------------------------------

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
       AND (
               has_table_privilege(role_name.rolname, 'public.organizations', 'INSERT')
            OR has_table_privilege(role_name.rolname, 'public.organizations', 'UPDATE')
            OR has_table_privilege(role_name.rolname, 'public.organizations', 'DELETE')
            OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'INSERT')
            OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'UPDATE')
            OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'DELETE')
           );
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'login roles still have write privileges on tenant tables: %', offending;
    END IF;
END
$$;

COMMIT;
