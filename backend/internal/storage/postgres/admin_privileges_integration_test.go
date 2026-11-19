package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/migrations"
)

// TestIntegrationProtectedTablePrivilegesAreRevoked proves that no login role
// keeps a mutation privilege on the protected tables, not even TRUNCATE, which
// row level security does not cover.
//
// The role is created with TRUNCATE only, which is the privilege 000004 did
// not detect, and the migrations must take it away.
func TestIntegrationProtectedTablePrivilegesAreRevoked(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	databaseName := createIntegrationDatabase(t, ctx, superuserURL, superuserPool, "")
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	plan, err := LoadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}

	privilegedPool := connect(t, databaseURL)
	defer privilegedPool.Close()

	// Apply every migration except the last one, so the role can be granted
	// the privilege that the last migration has to revoke.
	if len(plan) < 2 {
		t.Fatalf("migrations = %d, want at least 2", len(plan))
	}
	if _, err := Migrate(ctx, NewMigrationStore(privilegedPool), plan[:len(plan)-1]); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	truncateRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_truncate")
	if _, err := privilegedPool.Exec(ctx, "GRANT TRUNCATE ON public.organizations, public.organization_memberships TO "+truncateRole); err != nil {
		t.Fatalf("grant truncate: %v", err)
	}
	if _, err := privilegedPool.Exec(ctx, "GRANT TRUNCATE ON app.admin_audit_log TO "+truncateRole); err != nil {
		t.Fatalf("grant truncate on the audit trail: %v", err)
	}

	if _, err := Migrate(ctx, NewMigrationStore(privilegedPool), plan); err != nil {
		t.Fatalf("apply the last migration: %v", err)
	}

	var canTruncate bool
	if err := privilegedPool.QueryRow(ctx,
		"SELECT has_table_privilege($1, 'public.organizations', 'TRUNCATE')", truncateRole).Scan(&canTruncate); err != nil {
		t.Fatalf("read the truncate privilege: %v", err)
	}
	if canTruncate {
		t.Fatal("the role still has TRUNCATE on the tenant tables")
	}

	// The startup check has to reject a role that can truncate the tenant
	// tables, because row level security does not apply to TRUNCATE. The
	// grant is replayed here to simulate an operator who leaves the
	// privilege behind after the migration ran, which is exactly the case
	// the migration cannot prevent on its own.
	if _, err := privilegedPool.Exec(ctx, "GRANT TRUNCATE ON public.organizations TO "+truncateRole); err != nil {
		t.Fatalf("grant truncate again: %v", err)
	}

	truncateURL := withCredentials(superuserURL, truncateRole, integrationPassword, databaseName)
	truncatePool := connect(t, truncateURL)
	defer truncatePool.Close()

	verifier := NewRuntimeRoleVerifier(NewPgxRoleInspector(truncatePool))
	err = verifier.VerifyRuntimeRole(ctx)
	if err == nil {
		t.Fatal("err = nil, want the runtime role to be rejected")
	}
	if !errors.Is(err, ErrRuntimeRoleCanWrite) {
		t.Fatalf("err = %v, want %v", err, ErrRuntimeRoleCanWrite)
	}
}

// TestIntegrationBootstrapRejectsUnexpectedRoleMembership proves that the
// bootstrap fails closed when a login role reaches the administrative groups
// or the owner role through a chain of memberships.
//
// A direct grant would show up in pg_auth_members, but a chain such as
// "GRANT energyhub_owner TO helper; GRANT helper TO energyhub_runtime" does
// not, so the bootstrap resolves the whole chain.
func TestIntegrationBootstrapRejectsUnexpectedRoleMembership(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	databaseName := createIntegrationDatabase(t, ctx, superuserURL, superuserPool, "")
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	ownerHelper := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_owner_helper")
	if _, err := superuserPool.Exec(ctx, "GRANT "+integrationOwnerRole+" TO "+ownerHelper); err != nil {
		t.Fatalf("grant the owner role to the helper: %v", err)
	}
	if _, err := superuserPool.Exec(ctx, "GRANT "+ownerHelper+" TO "+integrationRuntimeRole); err != nil {
		t.Fatalf("grant the helper to the runtime role: %v", err)
	}

	err := applyBootstrapExpectingFailure(t, ctx, connect(t, databaseURL))
	if !strings.Contains(err.Error(), "unexpected role membership") {
		t.Fatalf("bootstrap error = %q, want an unexpected role membership error", err.Error())
	}

	// The chain has to be removed explicitly, and then the bootstrap passes
	// again.
	if _, err := superuserPool.Exec(ctx, "REVOKE "+ownerHelper+" FROM "+integrationRuntimeRole); err != nil {
		t.Fatalf("revoke the helper: %v", err)
	}
	applyBootstrap(t, ctx, connect(t, databaseURL))
}

// TestIntegrationBootstrapRejectsSharedAdminCredential proves that granting
// the administrative credential to another login role fails closed. The
// membership is transitive, so such a role would hold both administrative
// groups without appearing in the direct membership table.
func TestIntegrationBootstrapRejectsSharedAdminCredential(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	databaseName := createIntegrationDatabase(t, ctx, superuserURL, superuserPool, "")
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	sharedAdmin := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_shared_admin")
	if _, err := superuserPool.Exec(ctx, "GRANT "+integrationAdminRole+" TO "+sharedAdmin); err != nil {
		t.Fatalf("grant the admin credential: %v", err)
	}

	err := applyBootstrapExpectingFailure(t, ctx, connect(t, databaseURL))
	if !strings.Contains(err.Error(), "unexpected login roles with administrative privileges") {
		t.Fatalf("bootstrap error = %q, want an unexpected administrative role error", err.Error())
	}

	// The grant has to be removed explicitly, and then the bootstrap passes
	// again.
	if _, err := superuserPool.Exec(ctx, "REVOKE "+integrationAdminRole+" FROM "+sharedAdmin); err != nil {
		t.Fatalf("revoke the admin credential: %v", err)
	}
	applyBootstrap(t, ctx, connect(t, databaseURL))
}

// TestIntegrationAdminFunctionsAreOwnedByTheOwnerRole proves that the
// administrative functions belong to energyhub_owner, so the credential that
// installed them cannot replace their body.
func TestIntegrationAdminFunctionsAreOwnedByTheOwnerRole(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	databaseName := createIntegrationDatabase(t, ctx, superuserURL, superuserPool, "")
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	plan, err := LoadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	privilegedPool := connect(t, databaseURL)
	defer privilegedPool.Close()
	if _, err := Migrate(ctx, NewMigrationStore(privilegedPool), plan); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rows, err := privilegedPool.Query(ctx, `
		SELECT function.oid::regprocedure::text, owner.rolname
		  FROM pg_proc AS function
		  JOIN pg_namespace AS schema ON schema.oid = function.pronamespace
		  JOIN pg_roles AS owner ON owner.oid = function.proowner
		 WHERE schema.nspname = 'app'
		   AND function.proname IN ('create_organization', 'grant_membership', 'revoke_membership')`)
	if err != nil {
		t.Fatalf("read the function owners: %v", err)
	}
	defer rows.Close()
	found := map[string]string{}
	for rows.Next() {
		var function, owner string
		if err := rows.Scan(&function, &owner); err != nil {
			t.Fatalf("scan the function owner: %v", err)
		}
		found[function] = owner
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the function owners: %v", err)
	}

	// The three administrative functions have to exist. Without this check the
	// test would pass when the functions were missing, which is the opposite of
	// what it is meant to prove.
	for _, function := range []string{
		"app.create_organization(text,text)",
		"app.grant_membership(text,text,text)",
		"app.revoke_membership(text,text)",
	} {
		owner, ok := found[function]
		if !ok {
			t.Fatalf("the administrative function %s does not exist", function)
		}
		if owner != integrationOwnerRole {
			t.Fatalf("function %s is owned by %s, want %s", function, owner, integrationOwnerRole)
		}
	}
}

// applyBootstrapExpectingFailure runs the bootstrap and returns the error it
// produced.
func applyBootstrapExpectingFailure(t *testing.T, ctx context.Context, pool *pgxpool.Pool) error {
	t.Helper()

	statements, err := os.ReadFile(bootstrapSQLPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(statements), pgx.QueryExecModeSimpleProtocol); err == nil {
		t.Fatal("the bootstrap succeeded, want a failure")
	} else {
		return err
	}
	return nil
}
