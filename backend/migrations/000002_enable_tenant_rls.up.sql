ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
ALTER TABLE organization_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_memberships FORCE ROW LEVEL SECURITY;

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

CREATE POLICY organizations_write_admin ON organizations
    FOR ALL
    USING (current_setting('app.is_system_admin', true) = 'on')
    WITH CHECK (current_setting('app.is_system_admin', true) = 'on');

CREATE POLICY memberships_select_own ON organization_memberships
    FOR SELECT
    USING (
        subject = current_setting('app.current_subject', true)
        OR current_setting('app.is_system_admin', true) = 'on'
    );

CREATE POLICY memberships_write_admin ON organization_memberships
    FOR ALL
    USING (current_setting('app.is_system_admin', true) = 'on')
    WITH CHECK (current_setting('app.is_system_admin', true) = 'on');
