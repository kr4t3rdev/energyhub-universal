package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var hardenDownMigrationPath = "../../../migrations/000004_harden_admin_role_boundary.down.sql"

// TestIntegrationDownMigrationsExecute proves that the rollback files run
// against a real database. The 000003 rollback used to fail because it revoked
// privileges on functions it had already dropped, and it left the API without
// read access.
func TestIntegrationDownMigrationsExecute(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	environment := newIntegrationEnvironment(t, superuserURL)
	ctx := context.Background()

	adminPool := connect(t, environment.adminURL)
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-rollback', 'Rollback Org')"); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-rollback', 'dev|user-a', 'viewer')"); err != nil {
		t.Fatalf("grant membership: %v", err)
	}

	// The rollback files are a manual operation for an operator, so they run
	// with a privileged connection.
	privilegedPool := connect(t, withDatabase(superuserURL, environment.databaseName))
	defer privilegedPool.Close()

	t.Run("the 000004 rollback restores the previous state", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, hardenDownMigrationPath)

		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM app.admin_audit_log"); err == nil {
			t.Fatal("audit trail still exists after the rollback")
		}

		rows, err := adminPool.Query(ctx, "SELECT subject FROM public.organization_memberships")
		if err != nil {
			t.Fatalf("admin read after rollback: %v", err)
		}
		rows.Close()
	})

	t.Run("the 000003 rollback removes the admin functions and keeps the reads", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, adminWriteDownMigrationPath)

		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-after', 'After')"); err == nil {
			t.Fatal("admin function still exists after the rollback")
		} else if !strings.Contains(err.Error(), "app") {
			t.Fatalf("err = %v, want a missing schema error", err)
		}

		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		reader := NewMembershipReader(NewDB(runtimePool))
		organizations, err := reader.ListOrganizations(ctx, "dev|user-a")
		if err != nil {
			t.Fatalf("read after rollback: %v", err)
		}
		if len(organizations) != 1 || organizations[0].ID != "org-rollback" {
			t.Fatalf("organizations = %#v, want the organization of the subject", organizations)
		}
	})
}

func executeSQLFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, path string) {
	t.Helper()

	statements, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(statements), pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatalf("execute %s: %v", path, err)
	}
}
