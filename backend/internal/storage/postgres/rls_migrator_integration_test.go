package postgres

import (
	"context"
	"testing"

	"energyhub-universal/backend/migrations"
)

// TestIntegrationMigrationWithNonSuperuserOwner proves that the migrations can
// be applied by a dedicated migrator that is not a superuser but is a member
// of energyhub_owner. The documented alternative to a superuser must actually
// work, including the ownership transfers of migration 000004.
func TestIntegrationMigrationWithNonSuperuserOwner(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	migratorRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_migrator", integrationOwnerRole)
	databaseName := createIntegrationDatabase(t, ctx, superuserPool, migratorRole)
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	plan, err := LoadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}

	migratorURL := withCredentials(superuserURL, migratorRole, integrationPassword, databaseName)
	migratorPool := connect(t, migratorURL)
	defer migratorPool.Close()

	if _, err := Migrate(ctx, NewMigrationStore(migratorPool), plan); err != nil {
		t.Fatalf("apply migrations as non-superuser owner: %v", err)
	}

	t.Run("owner role owns the protected objects", func(t *testing.T) {
		for _, relation := range []string{"public.organizations", "public.organization_memberships"} {
			var owner string
			query := "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid = '" + relation + "'::regclass"
			if err := migratorPool.QueryRow(ctx, query).Scan(&owner); err != nil {
				t.Fatalf("read owner of %s: %v", relation, err)
			}
			if owner != integrationOwnerRole {
				t.Fatalf("owner of %s = %q, want %q", relation, owner, integrationOwnerRole)
			}
		}

		for _, function := range []string{
			"app.create_organization(text, text)",
			"app.grant_membership(text, text, text)",
			"app.revoke_membership(text, text)",
		} {
			var owner string
			query := "SELECT pg_get_userbyid(proowner) FROM pg_proc WHERE oid = '" + function + "'::regprocedure"
			if err := migratorPool.QueryRow(ctx, query).Scan(&owner); err != nil {
				t.Fatalf("read owner of %s: %v", function, err)
			}
			if owner != integrationOwnerRole {
				t.Fatalf("owner of %s = %q, want %q", function, owner, integrationOwnerRole)
			}
		}
	})

	t.Run("admin writes still work through the functions", func(t *testing.T) {
		adminRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_admin", integrationAdminGroup, integrationCapabilityRole)
		adminURL := withCredentials(superuserURL, adminRole, integrationPassword, databaseName)
		adminPool := connect(t, adminURL)
		defer adminPool.Close()

		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-migrator', 'Migrator Org')"); err != nil {
			t.Fatalf("create organization: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-migrator', 'dev|user-a', 'viewer')"); err != nil {
			t.Fatalf("grant membership: %v", err)
		}
	})
}
