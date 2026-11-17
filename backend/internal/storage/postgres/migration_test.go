package postgres

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

var migrationPath = "../../../migrations/000001_create_organizations_and_memberships.up.sql"
var rlsMigrationPath = "../../../migrations/000002_enable_tenant_rls.up.sql"
var rlsDownMigrationPath = "../../../migrations/000002_enable_tenant_rls.down.sql"
var adminWriteMigrationPath = "../../../migrations/000003_secure_admin_writes.up.sql"
var adminWriteDownMigrationPath = "../../../migrations/000003_secure_admin_writes.down.sql"

func TestMigrationKeepsMembershipsTenantScoped(t *testing.T) {
	schema, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)

	for _, required := range []string{
		"organization_id TEXT NOT NULL",
		"subject TEXT NOT NULL",
		"PRIMARY KEY (organization_id, subject)",
		"ON DELETE CASCADE",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration is missing %q", required)
		}
	}
}

func TestMigrationRoleCheckMatchesContractRoles(t *testing.T) {
	schema, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}

	match := regexp.MustCompile(`role IN \(([^)]*)\)`).FindStringSubmatch(string(schema))
	if match == nil {
		t.Fatal("migration has no role CHECK constraint")
	}

	sqlRoles := map[string]bool{}
	for _, role := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(match[1], -1) {
		sqlRoles[role[1]] = true
	}

	contractRoles := []contracts.OrganizationRole{
		contracts.RoleOwner,
		contracts.RoleAdmin,
		contracts.RoleMember,
		contracts.RoleViewer,
	}
	if len(sqlRoles) != len(contractRoles) {
		t.Fatalf("migration roles = %#v, want %d roles", sqlRoles, len(contractRoles))
	}
	for _, role := range contractRoles {
		if !sqlRoles[string(role)] {
			t.Fatalf("migration is missing contract role %q", role)
		}
	}
}

func TestAdminWriteMigrationReplacesForgeableGUCBoundary(t *testing.T) {
	schema, err := os.ReadFile(adminWriteMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)

	for _, required := range []string{
		"CREATE SCHEMA IF NOT EXISTS app",
		"CREATE OR REPLACE FUNCTION app.grant_membership",
		"CREATE OR REPLACE FUNCTION app.revoke_membership",
		"CREATE OR REPLACE FUNCTION app.create_organization",
		"SECURITY DEFINER",
		"SET search_path = ''",
		"pg_has_role(session_user, 'energyhub_membership_admin', 'member')",
		"REVOKE ALL ON FUNCTION app.grant_membership",
		"REVOKE ALL ON FUNCTION app.revoke_membership",
		"REVOKE ALL ON FUNCTION app.create_organization",
		"FROM PUBLIC",
		"GRANT EXECUTE ON FUNCTION app.grant_membership",
		"GRANT EXECUTE ON FUNCTION app.revoke_membership",
		"GRANT EXECUTE ON FUNCTION app.create_organization",
		"TO energyhub_membership_admin",
		"ALTER FUNCTION app.grant_membership",
		"ALTER FUNCTION app.revoke_membership",
		"ALTER FUNCTION app.create_organization",
		"OWNER TO energyhub_owner",
		"GRANT SELECT ON public.organizations, public.organization_memberships TO energyhub_runtime",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON public.organizations, public.organization_memberships TO energyhub_owner",
		"DROP POLICY IF EXISTS organizations_write_admin",
		"DROP POLICY IF EXISTS memberships_write_admin",
		"pg_has_role(current_user, 'energyhub_membership_admin', 'member')",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("admin write migration is missing %q", required)
		}
	}

	if strings.Contains(executableSQL(sql), "app.is_system_admin") {
		t.Fatal("admin write migration still trusts the forgeable app.is_system_admin setting")
	}
	if strings.Contains(executableSQL(sql), "GRANT ALL") {
		t.Fatal("admin write migration grants broad privileges")
	}
}

// executableSQL removes line comments so assertions look at the statements the
// database actually runs.
func executableSQL(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if index := strings.Index(line, "--"); index >= 0 {
			line = line[:index]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestAdminWriteDownMigrationRemovesSecureFunctions(t *testing.T) {
	schema, err := os.ReadFile(adminWriteDownMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)

	for _, required := range []string{
		"DROP FUNCTION IF EXISTS app.grant_membership",
		"DROP FUNCTION IF EXISTS app.revoke_membership",
		"DROP FUNCTION IF EXISTS app.create_organization",
		"DROP SCHEMA IF EXISTS app",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("admin write down migration is missing %q", required)
		}
	}
}

func TestRLSMigrationRestrictsReadsAndAdminWrites(t *testing.T) {
	schema, err := os.ReadFile(rlsMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)

	for _, required := range []string{
		"ALTER TABLE organizations ENABLE ROW LEVEL SECURITY",
		"ALTER TABLE organizations FORCE ROW LEVEL SECURITY",
		"ALTER TABLE organization_memberships ENABLE ROW LEVEL SECURITY",
		"ALTER TABLE organization_memberships FORCE ROW LEVEL SECURITY",
		"CREATE POLICY organizations_select_member",
		"CREATE POLICY organizations_write_admin",
		"CREATE POLICY memberships_select_own",
		"CREATE POLICY memberships_write_admin",
		"current_setting('app.current_subject', true)",
		"current_setting('app.is_system_admin', true) = 'on'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("RLS migration is missing %q", required)
		}
	}

	if strings.Contains(sql, "USING (true)") {
		t.Fatal("RLS migration contains a permissive USING (true) policy")
	}
}

func TestRLSDownMigrationRemovesPolicies(t *testing.T) {
	schema, err := os.ReadFile(rlsDownMigrationPath)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)

	for _, policy := range []string{
		"memberships_write_admin",
		"memberships_select_own",
		"organizations_write_admin",
		"organizations_select_member",
	} {
		if !strings.Contains(sql, "DROP POLICY IF EXISTS "+policy) {
			t.Fatalf("RLS down migration does not drop %s", policy)
		}
	}
	for _, table := range []string{"organizations", "organization_memberships"} {
		if !strings.Contains(sql, "ALTER TABLE "+table+" DISABLE ROW LEVEL SECURITY") {
			t.Fatalf("RLS down migration does not disable RLS on %s", table)
		}
	}
}
