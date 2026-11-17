package postgres

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

var migrationPath = "../../../migrations/000001_create_organizations_and_memberships.up.sql"

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
