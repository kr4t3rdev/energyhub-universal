-- Roll back the role-based administrative write boundary.
--
-- This restores the 000002 policy set, including the app.is_system_admin write
-- policies. That boundary is weaker: any role can set the custom GUC. Only use
-- this down migration to return to 000002 in a controlled environment, and
-- re-apply 000003 before the system handles real tenants.

DROP FUNCTION IF EXISTS app.create_organization(text, text);
DROP FUNCTION IF EXISTS app.grant_membership(text, text, text);
DROP FUNCTION IF EXISTS app.revoke_membership(text, text);
DROP SCHEMA IF EXISTS app;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_runtime') THEN
        REVOKE SELECT ON public.organizations, public.organization_memberships FROM energyhub_runtime;
        REVOKE USAGE ON SCHEMA public FROM energyhub_runtime;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_membership_admin') THEN
        REVOKE SELECT ON public.organizations, public.organization_memberships FROM energyhub_membership_admin;
        REVOKE USAGE ON SCHEMA public FROM energyhub_membership_admin;
        REVOKE EXECUTE ON FUNCTION app.create_organization(text, text) FROM energyhub_membership_admin;
        REVOKE EXECUTE ON FUNCTION app.grant_membership(text, text, text) FROM energyhub_membership_admin;
        REVOKE EXECUTE ON FUNCTION app.revoke_membership(text, text) FROM energyhub_membership_admin;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_owner') THEN
        REVOKE SELECT, INSERT, UPDATE, DELETE ON public.organizations, public.organization_memberships FROM energyhub_owner;
    END IF;
END
$$;

DROP POLICY IF EXISTS organizations_select_member ON organizations;
CREATE POLICY organizations_select_member ON organizations
    FOR SELECT
    USING (
        current_setting('app.is_system_admin', true) = 'on'
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
        OR current_setting('app.is_system_admin', true) = 'on'
    );

CREATE POLICY organizations_write_admin ON organizations
    FOR ALL
    USING (current_setting('app.is_system_admin', true) = 'on')
    WITH CHECK (current_setting('app.is_system_admin', true) = 'on');

CREATE POLICY memberships_write_admin ON organization_memberships
    FOR ALL
    USING (current_setting('app.is_system_admin', true) = 'on')
    WITH CHECK (current_setting('app.is_system_admin', true) = 'on');
