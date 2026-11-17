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
