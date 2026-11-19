package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/migrations"
)

var hardenDownMigrationPath = "../../../migrations/000004_harden_admin_role_boundary.down.sql"
var leftoverPrivilegeDownMigrationPath = "../../../migrations/000005_revoke_leftover_protected_table_privileges.down.sql"
var auditReadDownMigrationPath = "../../../migrations/000006_close_admin_audit_read.down.sql"

// TestIntegrationDownMigrationsExecute proves that the rollback files run
// against a real database and that they do not reopen the boundary they were
// written to close.
//
// The rollbacks are manual operations for an operator, so they run with a
// privileged connection. They are not allowed to restore the forgeable write
// policies, to give the administrative credential the tenant tables back or to
// remove the audit trail: an incident rollback is exactly the moment where an
// unnoticed weaker boundary would do the most damage.
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

	t.Run("the 000004 rollback keeps the hardened boundary", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, hardenDownMigrationPath)

		// The administrative functions are still there and still require the
		// capability role.
		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-after', 'After')"); err != nil {
			t.Fatalf("create organization after the rollback: %v", err)
		}

		// The administrative credential still cannot read the tenant tables.
		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM public.organization_memberships"); err == nil {
			t.Fatal("the administrative credential can read the tenant tables after the rollback")
		} else {
			assertPermissionDenied(t, err)
		}

		// The audit trail is still there.
		var audited int
		if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM app.admin_audit_log").Scan(&audited); err != nil {
			t.Fatalf("read the audit trail: %v", err)
		}
		if audited == 0 {
			t.Fatal("the audit trail is empty after the rollback")
		}
	})

	t.Run("the 000003 rollback does not restore the forgeable policies", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, adminWriteDownMigrationPath)

		// The functions are still there, the write path still works.
		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-rollback', 'dev|user-b', 'member')"); err != nil {
			t.Fatalf("grant membership after the rollback: %v", err)
		}

		// The forgeable write policies are not back: the runtime credential
		// still cannot write.
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		if _, err := runtimePool.Exec(ctx, "INSERT INTO public.organizations (id, name) VALUES ('org-direct', 'Direct')"); err == nil {
			t.Fatal("the runtime credential can write after the rollback")
		} else {
			assertPermissionDenied(t, err)
		}

		// The reads still work.
		reader := NewMembershipReader(NewDB(runtimePool))
		organizations, err := reader.ListOrganizations(ctx, "dev|user-a")
		if err != nil {
			t.Fatalf("read after rollback: %v", err)
		}
		if len(organizations) != 1 || organizations[0].ID != "org-rollback" {
			t.Fatalf("organizations = %#v, want the organization of the subject", organizations)
		}
	})

	t.Run("the 000006 rollback keeps the audit trail closed", func(t *testing.T) {
		executeSQLFile(t, ctx, privilegedPool, auditReadDownMigrationPath)

		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM app.admin_audit_log"); err == nil {
			t.Fatal("the administrative credential can read the audit trail after the rollback")
		} else {
			assertPermissionDenied(t, err)
		}
	})
}

// TestIntegrationAdminWriteRollbackKeepsTheAuditTrail proves that no rollback
// erases the evidence of who changed the memberships. The audit trail is
// verified in place, before and after the rollback files run.
func TestIntegrationAdminWriteRollbackKeepsTheAuditTrail(t *testing.T) {
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

	var before int
	if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM app.admin_audit_log").Scan(&before); err != nil {
		t.Fatalf("read the audit trail: %v", err)
	}
	if before == 0 {
		t.Fatal("the audit trail is empty before the rollback")
	}

	executeSQLFile(t, ctx, privilegedPool, adminWriteDownMigrationPath)
	executeSQLFile(t, ctx, privilegedPool, hardenDownMigrationPath)

	var after int
	if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM app.admin_audit_log").Scan(&after); err != nil {
		t.Fatalf("read the audit trail: %v", err)
	}
	if after != before {
		t.Fatalf("audit entries = %d after the rollback, want %d", after, before)
	}

	// The records are the ones of the administrative writes, with the actor
	// that made them.
	var actor, operation string
	if err := privilegedPool.QueryRow(ctx,
		"SELECT actor, operation FROM app.admin_audit_log ORDER BY id LIMIT 1").Scan(&actor, &operation); err != nil {
		t.Fatalf("read the audit record: %v", err)
	}
	if actor == "" || operation != "create_organization" {
		t.Fatalf("audit record = %q/%q, want the create organization record", actor, operation)
	}
}

// TestIntegrationMigrationsAreForwardOnly proves that no published migration
// changes the state of a database that already applied it. The check is on
// the up files: they are the ones the runner applies.
func TestIntegrationMigrationsAreForwardOnly(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	environment := newIntegrationEnvironment(t, superuserURL)
	ctx := context.Background()

	privilegedPool := connect(t, withDatabase(superuserURL, environment.databaseName))
	defer privilegedPool.Close()

	var applied int
	if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatalf("read the applied migrations: %v", err)
	}
	if applied == 0 {
		t.Fatal("no migration was applied")
	}

	// Applying the plan again is a no-op: the runner skips the versions that
	// are already in the table.
	plan, err := LoadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Migrate(ctx, NewMigrationStore(privilegedPool), plan)
	if err != nil {
		t.Fatalf("apply the migrations again: %v", err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("applied = %#v, want none on the second run", result.Applied)
	}
	if len(result.Skipped) != applied {
		t.Fatalf("skipped = %d, want %d", len(result.Skipped), applied)
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
