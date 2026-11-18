package postgres

import (
	"context"
	"testing"

	"energyhub-universal/backend/migrations"
)

// TestIntegrationRuntimeRoleCannotKeepTableOwnership reproduces the legacy
// topology the repository used to support: the credential that applied the
// migrations owned the tables, and a table owner can disable row level
// security, drop policies or grant itself write privileges.
//
// Migration 000004 must transfer that ownership to energyhub_owner and revoke
// the previous privileges, so the credential that applied the migrations can
// only read.
func TestIntegrationRuntimeRoleCannotKeepTableOwnership(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	legacyOwner := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_legacy_owner")
	databaseName := createIntegrationDatabase(t, ctx, superuserURL, superuserPool, legacyOwner)
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	plan, err := LoadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) < 4 {
		t.Fatalf("migrations = %d, want at least 4", len(plan))
	}

	// Legacy topology: the credential that applied the first migrations owns
	// the tables.
	legacyURL := withCredentials(superuserURL, legacyOwner, integrationPassword, databaseName)
	legacyPool := connect(t, legacyURL)
	defer legacyPool.Close()
	if _, err := Migrate(ctx, NewMigrationStore(legacyPool), plan[:2]); err != nil {
		t.Fatalf("apply legacy migrations as table owner: %v", err)
	}

	privilegedPool := connect(t, databaseURL)
	defer privilegedPool.Close()
	if _, err := Migrate(ctx, NewMigrationStore(privilegedPool), plan); err != nil {
		t.Fatalf("apply secure admin migrations: %v", err)
	}

	t.Run("tables and schema belong to energyhub_owner", func(t *testing.T) {
		for _, relation := range []string{"public.organizations", "public.organization_memberships"} {
			var owner string
			query := "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid = '" + relation + "'::regclass"
			if err := privilegedPool.QueryRow(ctx, query).Scan(&owner); err != nil {
				t.Fatalf("read owner of %s: %v", relation, err)
			}
			if owner != integrationOwnerRole {
				t.Fatalf("owner of %s = %q, want %q", relation, owner, integrationOwnerRole)
			}
		}

		var schemaOwner string
		if err := privilegedPool.QueryRow(ctx, "SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'app'").Scan(&schemaOwner); err != nil {
			t.Fatalf("read owner of app schema: %v", err)
		}
		if schemaOwner != integrationOwnerRole {
			t.Fatalf("owner of app schema = %q, want %q", schemaOwner, integrationOwnerRole)
		}
	})

	t.Run("the previous owner cannot change the schema", func(t *testing.T) {
		statements := []string{
			"ALTER TABLE public.organizations DISABLE ROW LEVEL SECURITY",
			"ALTER TABLE public.organizations ENABLE ROW LEVEL SECURITY",
			"ALTER TABLE public.organizations ADD COLUMN leaked text",
			"ALTER TABLE public.organizations OWNER TO " + legacyOwner,
			"DROP POLICY organizations_select_member ON public.organizations",
			"DROP FUNCTION app.grant_membership(text, text, text)",
		}
		for _, statement := range statements {
			if _, err := legacyPool.Exec(ctx, statement); err == nil {
				t.Fatalf("statement %q succeeded, want permission denied", statement)
			}
		}
	})

	t.Run("the previous owner cannot write", func(t *testing.T) {
		if _, err := legacyPool.Exec(ctx, "INSERT INTO public.organizations (id, name) VALUES ('org-owned', 'Owned')"); err == nil {
			t.Fatal("insert succeeded, want permission denied")
		}
		if _, err := legacyPool.Exec(ctx, "SELECT app.grant_membership('org-1', 'dev|user-b', 'member')"); err == nil {
			t.Fatal("admin function call succeeded, want permission denied")
		}
	})

	t.Run("the runtime credential cannot write either", func(t *testing.T) {
		runtimeRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_runtime", integrationRuntimeRole)
		runtimeURL := withCredentials(superuserURL, runtimeRole, integrationPassword, databaseName)
		runtimePool := connect(t, runtimeURL)
		defer runtimePool.Close()

		if _, err := runtimePool.Exec(ctx, "INSERT INTO public.organizations (id, name) VALUES ('org-runtime', 'Runtime')"); err == nil {
			t.Fatal("insert succeeded, want permission denied")
		}
		if _, err := runtimePool.Exec(ctx, "ALTER TABLE public.organizations DISABLE ROW LEVEL SECURITY"); err == nil {
			t.Fatal("disable row level security succeeded, want permission denied")
		}

		subjects := readSubjects(t, runtimePool, "dev|user-a")
		if len(subjects) != 0 {
			t.Fatalf("subjects = %#v, want none", subjects)
		}
	})
}
