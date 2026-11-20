-- Create the sites and site_devices tables with tenant-scoped RLS and the
-- audited administrative write functions.
--
-- Sites and devices are created by the administrative CLI only: there is no
-- HTTP write surface. Reads go through the runtime credential with the
-- membership policies below, so a subject only sees the sites of the
-- organizations it belongs to. There is no readings table in this migration:
-- telemetry is postponed by design, so overviews report no readings yet.

BEGIN;

-- Audit trail ---------------------------------------------------------------

-- The audit CHECK lists the operations the administrative functions record.
-- The new operations are added by a new migration on purpose: 000004 is
-- already applied and a published migration is never edited.
ALTER TABLE app.admin_audit_log DROP CONSTRAINT IF EXISTS admin_audit_log_operation_check;
ALTER TABLE app.admin_audit_log ADD CONSTRAINT admin_audit_log_operation_check
    CHECK (operation IN ('create_organization', 'grant_membership', 'revoke_membership', 'create_site', 'create_site_device'));

-- Tables --------------------------------------------------------------------

CREATE TABLE public.sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    timezone TEXT NOT NULL CHECK (length(btrim(timezone)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sites_organization_id_idx ON public.sites (organization_id, name, id);

CREATE TABLE public.site_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    kind TEXT NOT NULL CHECK (length(btrim(kind)) > 0),
    status TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX site_devices_site_id_idx ON public.site_devices (site_id, name, id);

ALTER TABLE public.sites OWNER TO energyhub_owner;
ALTER TABLE public.site_devices OWNER TO energyhub_owner;

-- Row level security ---------------------------------------------------------

ALTER TABLE public.sites ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sites FORCE ROW LEVEL SECURITY;
ALTER TABLE public.site_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.site_devices FORCE ROW LEVEL SECURITY;

CREATE POLICY sites_select_member ON public.sites
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1
            FROM public.organization_memberships AS membership
            WHERE membership.organization_id = public.sites.organization_id
              AND membership.subject = current_setting('app.current_subject', true)
        )
    );

CREATE POLICY site_devices_select_member ON public.site_devices
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1
            FROM public.sites AS site
            JOIN public.organization_memberships AS membership
              ON membership.organization_id = site.organization_id
            WHERE site.id = public.site_devices.site_id
              AND membership.subject = current_setting('app.current_subject', true)
        )
    );

-- Privileges -----------------------------------------------------------------

REVOKE ALL ON public.sites, public.site_devices FROM PUBLIC;

GRANT USAGE ON SCHEMA public TO energyhub_runtime;
GRANT SELECT ON public.sites, public.site_devices TO energyhub_runtime;

REVOKE CREATE ON SCHEMA public FROM energyhub_runtime, energyhub_admin, energyhub_membership_admin;

-- Administrative functions ---------------------------------------------------

CREATE OR REPLACE FUNCTION app.create_site(p_site_id uuid, p_organization_id text, p_name text, p_timezone text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_admin_capability', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_site_id IS NULL THEN
        RAISE EXCEPTION 'site_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_name IS NULL OR btrim(p_name) = '' THEN
        RAISE EXCEPTION 'site name is required' USING ERRCODE = '22023';
    END IF;
    IF p_timezone IS NULL OR btrim(p_timezone) = '' THEN
        RAISE EXCEPTION 'site timezone is required' USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.sites (id, organization_id, name, timezone)
    VALUES (p_site_id, btrim(p_organization_id), btrim(p_name), btrim(p_timezone));

    INSERT INTO app.admin_audit_log (actor, operation, organization_id)
    VALUES (session_user, 'create_site', btrim(p_organization_id));
EXCEPTION
    WHEN unique_violation THEN
        RAISE EXCEPTION 'site % already exists', p_site_id USING ERRCODE = '23505';
END;
$$;

CREATE OR REPLACE FUNCTION app.create_site_device(p_device_id uuid, p_site_id uuid, p_organization_id text, p_name text, p_kind text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
    IF NOT pg_has_role(session_user, 'energyhub_admin_capability', 'member') THEN
        RAISE EXCEPTION 'caller is not a membership administrator' USING ERRCODE = '42501';
    END IF;
    IF p_device_id IS NULL THEN
        RAISE EXCEPTION 'device_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_site_id IS NULL THEN
        RAISE EXCEPTION 'site_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_organization_id IS NULL OR btrim(p_organization_id) = '' THEN
        RAISE EXCEPTION 'organization_id is required' USING ERRCODE = '22023';
    END IF;
    IF p_name IS NULL OR btrim(p_name) = '' THEN
        RAISE EXCEPTION 'device name is required' USING ERRCODE = '22023';
    END IF;
    IF p_kind IS NULL OR btrim(p_kind) = '' THEN
        RAISE EXCEPTION 'device kind is required' USING ERRCODE = '22023';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM public.sites
        WHERE id = p_site_id AND organization_id = btrim(p_organization_id)
    ) THEN
        RAISE EXCEPTION 'site % is not part of organization %', p_site_id, btrim(p_organization_id) USING ERRCODE = '22023';
    END IF;

    INSERT INTO public.site_devices (id, site_id, name, kind)
    VALUES (p_device_id, p_site_id, btrim(p_name), btrim(p_kind));

    INSERT INTO app.admin_audit_log (actor, operation, organization_id)
    VALUES (session_user, 'create_site_device', btrim(p_organization_id));
EXCEPTION
    WHEN unique_violation THEN
        RAISE EXCEPTION 'device % already exists', p_device_id USING ERRCODE = '23505';
END;
$$;

ALTER FUNCTION app.create_site(uuid, text, text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.create_site_device(uuid, uuid, text, text, text) OWNER TO energyhub_owner;

REVOKE ALL ON FUNCTION app.create_site(uuid, text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION app.create_site_device(uuid, uuid, text, text, text) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION app.create_site(uuid, text, text, text) TO energyhub_membership_admin;
GRANT EXECUTE ON FUNCTION app.create_site_device(uuid, uuid, text, text, text) TO energyhub_membership_admin;

-- Leftover privileges ---------------------------------------------------------

-- Any role other than the privileged migration credential loses the mutation
-- privileges on the new tables, including TRUNCATE, which RLS does not cover.
DO $$
DECLARE
    role_name text;
BEGIN
    FOR role_name IN
        SELECT rolname FROM pg_roles
         WHERE NOT rolsuper
           AND NOT pg_has_role(rolname, 'energyhub_owner', 'member')
    LOOP
        EXECUTE format(
            'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.sites, public.site_devices FROM %I',
            role_name);
    END LOOP;
END
$$;

-- Fail closed if a login role can still change the new tables, or if the new
-- functions are not owned by energyhub_owner.
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
       AND EXISTS (
           SELECT 1
             FROM unnest(ARRAY['public.sites', 'public.site_devices']) AS protected(table_name)
             CROSS JOIN unnest(ARRAY['INSERT', 'UPDATE', 'DELETE', 'TRUNCATE']) AS privilege(privilege_name)
            WHERE has_table_privilege(role_name.rolname, protected.table_name, privilege.privilege_name)
       );
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'login roles still have write privileges on the site tables: %', offending;
    END IF;

    SELECT string_agg(function.oid::regprocedure::text, ', ' ORDER BY function.oid::regprocedure::text)
      INTO offending
      FROM pg_proc AS function
      JOIN pg_namespace AS schema ON schema.oid = function.pronamespace
      JOIN pg_roles AS owner ON owner.oid = function.proowner
     WHERE schema.nspname = 'app'
       AND function.proname IN ('create_site', 'create_site_device')
       AND owner.rolname <> 'energyhub_owner';
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'the site functions are not owned by energyhub_owner: %', offending;
    END IF;
END
$$;

COMMIT;
