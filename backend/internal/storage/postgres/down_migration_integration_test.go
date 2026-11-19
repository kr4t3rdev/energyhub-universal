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
var leftoverPrivilegeDownMigrationPath = "../../../migrations/000005_revoke_leftover_protected_table_privileges.down.sql"

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

	t.Run("the 000005 rollback keeps the protected tables closed", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, leftoverPrivilegeDownMigrationPath)

		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		var canTruncate bool
		if err := runtimePool.QueryRow(ctx,
			"SELECT has_table_privilege(current_user, 'public.organizations', 'TRUNCATE')").Scan(&canTruncate); err != nil {
			t.Fatalf("read the truncate privilege: %v", err)
		}
		if canTruncate {
			t.Fatal("the runtime credential can truncate the tenant tables")
		}
	})

	t.Run("the 000004 rollback restores the previous state", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, hardenDownMigrationPath)

		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM app.admin_audit_log"); err == nil {
			t.Fatal("audit trail still exists after the rollback")
		}

		// The administrative write records are archived, not erased.
		var archived int
		if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM public.admin_audit_log_archive_000004").Scan(&archived); err != nil {
			t.Fatalf("read the audit archive: %v", err)
		}
		if archived == 0 {
			t.Fatal("the audit archive is empty after the rollback")
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

// TestIntegrationAdminWriteRollbackArchivesTheAuditTrail proves that the 000003
// rollback does not destroy the audit trail when it is used on its own. The
// 000004 rollback archives the audit trail as well, but an operator can roll
// back a single version, and the evidence of who changed the memberships has
// to survive that too.
func TestIntegrationAdminWriteRollbackArchivesTheAuditTrail(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	environment := newIntegrationEnvironment(t, superuserURL)
	ctx := context.Background()

	adminPool := connect(t, environment.adminURL)
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-archive', 'Archive Org')"); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-archive', 'dev|user-a', 'viewer')"); err != nil {
		t.Fatalf("grant membership: %v", err)
	}

	privilegedPool := connect(t, withDatabase(superuserURL, environment.databaseName))
	defer privilegedPool.Close()

	executeSQLFile(t, ctx, privilegedPool, adminWriteDownMigrationPath)

	// The audit trail is archived, not erased.
	var archived int
	if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM public.admin_audit_log_archive_000003").Scan(&archived); err != nil {
		t.Fatalf("read the audit archive: %v", err)
	}
	if archived == 0 {
		t.Fatal("the audit archive is empty after the rollback")
	}

	// The audit trail itself disappears with the app schema.
	if _, err := privilegedPool.Exec(ctx, "SELECT count(*) FROM app.admin_audit_log"); err == nil {
		t.Fatal("the audit trail still exists after the rollback")
	}
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
