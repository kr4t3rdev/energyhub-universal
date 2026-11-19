-- Roll back the role-based administrative write boundary.
--
-- This restores the 000002 policy set, including the app.is_system_admin write
-- policies. That boundary is weaker: any role can set the custom GUC. Only use
-- this down migration to return to 000002 in a controlled environment, and
-- re-apply 000003 and 000004 before the system handles real tenants.
--
-- The privileges are revoked before the functions and the schema disappear,
-- and the read access of the runtime role is restored, so a rollback leaves a
-- database the API can still read.

BEGIN;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_membership_admin') THEN
        REVOKE EXECUTE ON FUNCTION app.create_organization(text, text) FROM energyhub_membership_admin;
        REVOKE EXECUTE ON FUNCTION app.grant_membership(text, text, text) FROM energyhub_membership_admin;
        REVOKE EXECUTE ON FUNCTION app.revoke_membership(text, text) FROM energyhub_membership_admin;
        REVOKE ALL ON SCHEMA app FROM energyhub_membership_admin;
    END IF;
END
$$;

DROP FUNCTION IF EXISTS app.create_organization(text, text);
DROP FUNCTION IF EXISTS app.grant_membership(text, text, text);
DROP FUNCTION IF EXISTS app.revoke_membership(text, text);

-- The audit trail is archived before it is dropped. The 000004 rollback
-- archives it as well, but this file can be used on its own, and a rollback
-- must never erase the evidence of who changed the memberships.
DO $$
BEGIN
    IF to_regclass('app.admin_audit_log') IS NULL THEN
        RETURN;
    END IF;
    EXECUTE 'DROP TABLE IF EXISTS public.admin_audit_log_archive_000003';
    EXECUTE 'CREATE TABLE public.admin_audit_log_archive_000003 (LIKE app.admin_audit_log EXCLUDING IDENTITY)';
    EXECUTE 'INSERT INTO public.admin_audit_log_archive_000003 SELECT * FROM app.admin_audit_log';
END
$$;

DROP TABLE IF EXISTS app.admin_audit_log;
DROP SCHEMA IF EXISTS app;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_runtime') THEN
        GRANT USAGE ON SCHEMA public TO energyhub_runtime;
        GRANT SELECT ON public.organizations, public.organization_memberships TO energyhub_runtime;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_membership_admin') THEN
        REVOKE SELECT ON public.organizations, public.organization_memberships FROM energyhub_membership_admin;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_owner') THEN
        REVOKE SELECT, INSERT, UPDATE, DELETE ON public.organizations, public.organization_memberships FROM energyhub_owner;
    END IF;
END
$$;

DROP POLICY IF EXISTS organizations_select_member ON public.organizations;
CREATE POLICY organizations_select_member ON public.organizations
    FOR SELECT
    USING (
        current_setting('app.is_system_admin', true) = 'on'
        OR EXISTS (
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
        OR current_setting('app.is_system_admin', true) = 'on'
    );

CREATE POLICY organizations_write_admin ON public.organizations
    FOR ALL
    USING (current_setting('app.is_system_admin', true) = 'on')
    WITH CHECK (current_setting('app.is_system_admin', true) = 'on');

CREATE POLICY memberships_write_admin ON public.organization_memberships
    FOR ALL
    USING (current_setting('app.is_system_admin', true) = 'on')
    WITH CHECK (current_setting('app.is_system_admin', true) = 'on');

COMMIT;
