DROP POLICY IF EXISTS memberships_write_admin ON organization_memberships;
DROP POLICY IF EXISTS memberships_select_own ON organization_memberships;
DROP POLICY IF EXISTS organizations_write_admin ON organizations;
DROP POLICY IF EXISTS organizations_select_member ON organizations;

ALTER TABLE organization_memberships NO FORCE ROW LEVEL SECURITY;
ALTER TABLE organization_memberships DISABLE ROW LEVEL SECURITY;
ALTER TABLE organizations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE organizations DISABLE ROW LEVEL SECURITY;
