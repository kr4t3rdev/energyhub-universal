package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/contracts"
	"energyhub-universal/backend/migrations"
)

// The PostgreSQL integration test proves the tenant boundary against a real
// server. It is skipped unless TEST_DATABASE_URL points to a PostgreSQL
// superuser, because the test creates roles and a throwaway database.
//
//	TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55432/postgres' go test ./internal/storage/postgres
//
// The test is destructive on the cluster (it creates roles), so it refuses to
// run against a host that is not a loopback address unless the operator
// confirms the cluster is disposable.
const integrationSuperuserURLEnv = "TEST_DATABASE_URL"
const integrationAllowRemoteEnv = "TEST_DATABASE_ALLOW_REMOTE"

const (
	integrationRuntimeRole    = "energyhub_runtime"
	integrationAdminRole      = "energyhub_admin"
	integrationOwnerRole      = "energyhub_owner"
	integrationAdminGroup     = "energyhub_membership_admin"
	integrationCapabilityRole = "energyhub_admin_capability"
	integrationPassword       = "dev-integration"
)

var bootstrapSQLPath = "../../../deploy/postgres/bootstrap.sql"

type integrationEnvironment struct {
	runtimeURL   string
	adminURL     string
	runtimeRole  string
	databaseName string
}

func TestIntegrationTenantIsolationAndAdminWrites(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	environment := newIntegrationEnvironment(t, superuserURL)
	ctx := context.Background()

	t.Run("admin provisions organization and membership", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-1', 'Demo Org')"); err != nil {
			t.Fatalf("create organization: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-1', 'dev|user-a', 'viewer')"); err != nil {
			t.Fatalf("grant membership: %v", err)
		}
	})

	t.Run("runtime reads only its own memberships", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		reader := NewMembershipReader(NewDB(runtimePool))
		organizations, err := reader.ListOrganizations(ctx, "dev|user-a")
		if err != nil {
			t.Fatalf("list organizations: %v", err)
		}
		if len(organizations) != 1 || organizations[0].ID != "org-1" {
			t.Fatalf("organizations = %#v, want only org-1", organizations)
		}
		if organizations[0].Role != contracts.RoleViewer {
			t.Fatalf("role = %q, want %q", organizations[0].Role, contracts.RoleViewer)
		}

		others, err := reader.ListOrganizations(ctx, "dev|user-b")
		if err != nil {
			t.Fatalf("list organizations: %v", err)
		}
		if len(others) != 0 {
			t.Fatalf("organizations = %#v, want none for another subject", others)
		}
	})

	t.Run("runtime cannot write tables directly", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		_, err := runtimePool.Exec(ctx, "INSERT INTO public.organizations (id, name) VALUES ('org-direct', 'Direct')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)
	})

	t.Run("runtime cannot call the admin write functions", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		_, err := runtimePool.Exec(ctx, "SELECT app.grant_membership('org-1', 'dev|user-b', 'member')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)
	})

	t.Run("runtime cannot forge administrator reads", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		_, err := runtimePool.Exec(ctx, "SET app.is_system_admin = 'on'")
		if err != nil {
			t.Fatalf("set setting: %v", err)
		}
		subjects := readSubjects(t, runtimePool, "dev|user-a")
		if len(subjects) != 1 {
			t.Fatalf("subjects = %#v, want only the caller's own membership", subjects)
		}
	})

	t.Run("admin cannot write tables directly", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		_, err := adminPool.Exec(ctx, "INSERT INTO public.organizations (id, name) VALUES ('org-direct', 'Direct')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)
	})

	t.Run("admin cannot read the tenant tables", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		_, err := adminPool.Exec(ctx, "SELECT subject FROM public.organization_memberships")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)
	})

	t.Run("admin revokes a membership", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		if _, err := adminPool.Exec(ctx, "SELECT app.revoke_membership('org-1', 'dev|user-a')"); err != nil {
			t.Fatalf("revoke membership: %v", err)
		}
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		subjects := readSubjects(t, runtimePool, "dev|user-a")
		if len(subjects) != 0 {
			t.Fatalf("subjects = %#v, want none after revoke", subjects)
		}
	})

	t.Run("admin functions reject invalid input", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-1', 'dev|user-a', 'superuser')"); err == nil {
			t.Fatal("err = nil, want invalid role rejection")
		}
		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('', 'dev|user-a', 'viewer')"); err == nil {
			t.Fatal("err = nil, want empty organization rejection")
		}
	})

	t.Run("creating an existing organization is a conflict", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		_, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-1', 'Renamed Org')")
		if err == nil {
			t.Fatal("err = nil, want conflict error")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("err = %v, want an already exists conflict", err)
		}

		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()
		reader := NewMembershipReader(NewDB(runtimePool))
		if _, err := reader.ListOrganizations(ctx, "dev|user-a"); err != nil {
			t.Fatalf("list organizations: %v", err)
		}
	})

	t.Run("administrative writes are audited", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		// The administrative credential writes through the functions but does
		// not read the audit trail: the whole administrative history of every
		// tenant is not part of its job.
		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM app.admin_audit_log"); err == nil {
			t.Fatal("err = nil, want the audit trail to be out of reach")
		} else {
			assertPermissionDenied(t, err)
		}

		privilegedPool := connect(t, withDatabase(superuserURL, environment.databaseName))
		defer privilegedPool.Close()

		rows, err := privilegedPool.Query(ctx, "SELECT operation, organization_id, subject, role FROM app.admin_audit_log ORDER BY id")
		if err != nil {
			t.Fatalf("read audit trail: %v", err)
		}
		defer rows.Close()

		entries := []string{}
		for rows.Next() {
			var operation, organizationID string
			var subject, role *string
			if err := rows.Scan(&operation, &organizationID, &subject, &role); err != nil {
				t.Fatal(err)
			}
			entry := operation + ":" + organizationID
			if subject != nil {
				entry += ":" + *subject
			}
			if role != nil {
				entry += ":" + *role
			}
			entries = append(entries, entry)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}

		want := []string{
			"create_organization:org-1",
			"grant_membership:org-1:dev|user-a:viewer",
			"revoke_membership:org-1:dev|user-a",
		}
		if len(entries) != len(want) {
			t.Fatalf("audit entries = %#v, want %#v", entries, want)
		}
		for index, entry := range want {
			if entries[index] != entry {
				t.Fatalf("audit entry %d = %q, want %q", index, entries[index], entry)
			}
		}
	})
}

// TestIntegrationDocumentsForgeableTenantContext records the known soft
// boundary of the read isolation: the runtime credential can set the tenant
// context itself, because a custom GUC cannot be protected in PostgreSQL.
//
// The application always binds the verified subject, so this only matters if
// the database credential leaks or an injection reaches SQL. The test fails on
// purpose when the behaviour changes, so the documentation and the threat
// model are revisited instead of silently drifting.
func TestIntegrationDocumentsForgeableTenantContext(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	environment := newIntegrationEnvironment(t, superuserURL)
	ctx := context.Background()

	adminPool := connect(t, environment.adminURL)
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-victim', 'Victim Org')"); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-victim', 'dev|victim', 'owner')"); err != nil {
		t.Fatalf("grant membership: %v", err)
	}

	runtimePool := connect(t, environment.runtimeURL)
	defer runtimePool.Close()

	subjects := readSubjects(t, runtimePool, "dev|victim")
	if len(subjects) != 1 || subjects[0] != "dev|victim" {
		t.Fatalf("subjects = %#v, want the forged subject to read another tenant", subjects)
	}
	t.Log("known limitation: the runtime credential can forge app.current_subject; " +
		"the database credential is the trust boundary for reads")
}

// readSubjects reads the membership table with the tenant context of the given
// subject. It uses a single connection and a transaction local setting, the
// same way the production reader does.
func readSubjects(t *testing.T, pool *pgxpool.Pool, subject string) []string {
	t.Helper()
	ctx := context.Background()

	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Release()

	tx, err := connection.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_subject', $1, true)", subject); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, "SELECT subject FROM public.organization_memberships")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	subjects := []string{}
	for rows.Next() {
		var scanned string
		if err := rows.Scan(&scanned); err != nil {
			t.Fatal(err)
		}
		subjects = append(subjects, scanned)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return subjects
}

func assertPermissionDenied(t *testing.T, err error) {
	t.Helper()
	message := err.Error()
	if !strings.Contains(message, "permission denied") && !strings.Contains(message, "row-level security") {
		t.Fatalf("err = %v, want a permission or row-level security error", err)
	}
}

func connect(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

// integrationSuperuserURL returns the configured superuser connection and
// refuses to run the destructive test against a shared cluster.
func integrationSuperuserURL(t *testing.T) string {
	t.Helper()
	superuserURL := strings.TrimSpace(os.Getenv(integrationSuperuserURLEnv))
	if superuserURL == "" {
		t.Skipf("%s is not set; skipping PostgreSQL integration test", integrationSuperuserURLEnv)
	}

	parsed, err := url.Parse(superuserURL)
	if err != nil {
		t.Fatalf("parse %s: %v", integrationSuperuserURLEnv, err)
	}
	host := parsed.Hostname()
	if !isLoopbackIP(host) && strings.TrimSpace(os.Getenv(integrationAllowRemoteEnv)) != "1" {
		t.Fatalf("refusing to run the destructive integration test against %q; "+
			"set %s=1 only when the cluster is disposable", host, integrationAllowRemoteEnv)
	}
	return superuserURL
}

// isLoopbackIP reports whether the host is a literal loopback address.
func isLoopbackIP(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// newIntegrationEnvironment creates the shared roles, a throwaway database and
// applies every migration to it. The test connects with ephemeral login roles
// so the credentials of the deployed roles are never changed.
func newIntegrationEnvironment(t *testing.T, superuserURL string) integrationEnvironment {
	t.Helper()
	ctx := context.Background()

	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	databaseName := createIntegrationDatabase(t, ctx, superuserURL, superuserPool, "")
	databaseURL := withDatabase(superuserURL, databaseName)

	applyBootstrap(t, ctx, connect(t, databaseURL))

	runtimeRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_runtime", integrationRuntimeRole)
	adminRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_admin", integrationAdminGroup, integrationCapabilityRole)

	migratorPool := connect(t, databaseURL)
	defer migratorPool.Close()
	plan, err := LoadMigrations(migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, NewMigrationStore(migratorPool), plan); err != nil {
		t.Fatal(err)
	}

	return integrationEnvironment{
		runtimeURL:   withCredentials(superuserURL, runtimeRole, integrationPassword, databaseName),
		adminURL:     withCredentials(superuserURL, adminRole, integrationPassword, databaseName),
		runtimeRole:  runtimeRole,
		databaseName: databaseName,
	}
}

// integrationDatabaseName returns the name of the throwaway database of this
// test process. Every helper uses it, so a role created before the database
// still knows where it has to release the privileges it holds.
func integrationDatabaseName() string {
	return fmt.Sprintf("energyhub_it_%d", os.Getpid())
}

// createIntegrationDatabase creates a throwaway database and removes it when
// the test ends.
func createIntegrationDatabase(t *testing.T, ctx context.Context, superuserURL string, superuserPool *pgxpool.Pool, ownerRole string) string {
	t.Helper()

	databaseName := integrationDatabaseName()
	if _, err := superuserPool.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	statement := "CREATE DATABASE " + databaseName
	if ownerRole != "" {
		statement += " OWNER " + ownerRole
	}
	if _, err := superuserPool.Exec(ctx, statement); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupPool, err := pgxpool.New(context.Background(), superuserURL)
		if err != nil {
			t.Logf("connect for cleanup: %v", err)
			return
		}
		defer cleanupPool.Close()
		if _, err := cleanupPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+databaseName+" WITH (FORCE)"); err != nil {
			t.Logf("drop test database: %v", err)
		}
	})
	return databaseName
}

// applyBootstrap runs the deployment bootstrap against the test database. The
// script normalizes the cluster roles and creates the app schema, so the
// integration test exercises the same setup the deployment uses.
func applyBootstrap(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	statements, err := os.ReadFile(bootstrapSQLPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(statements), pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatalf("apply bootstrap: %v", err)
	}
}

// createEphemeralRole creates a login role that inherits the privileges of the
// given roles and removes it when the test ends. The deployed login roles keep
// their own credentials.
func createEphemeralRole(t *testing.T, ctx context.Context, superuserURL string, prefix string, inheritedRoles ...string) string {
	t.Helper()

	name := fmt.Sprintf("%s_%d", prefix, os.Getpid())
	pool := connect(t, superuserURL)
	defer pool.Close()

	if _, err := pool.Exec(ctx, "DROP ROLE IF EXISTS "+name); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE ROLE "+name+" LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE PASSWORD '"+integrationPassword+"'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		dropEphemeralRole(t, superuserURL, name)
	})
	for _, inherited := range inheritedRoles {
		if _, err := pool.Exec(ctx, "GRANT "+inherited+" TO "+name); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

// dropEphemeralRole removes a test role from the cluster.
//
// The role may hold a privilege on the tables of the test database, and a role
// with a pending privilege cannot be dropped while that table exists, so the
// privileges are released first. The database is already gone in some tests,
// and the failure of that step is expected there.
func dropEphemeralRole(t *testing.T, superuserURL string, name string) {
	t.Helper()

	if pool, err := pgxpool.New(context.Background(), withDatabase(superuserURL, integrationDatabaseName())); err == nil {
		_, _ = pool.Exec(context.Background(), "DROP OWNED BY "+name+" CASCADE")
		pool.Close()
	}

	pool, err := pgxpool.New(context.Background(), superuserURL)
	if err != nil {
		t.Logf("connect for cleanup: %v", err)
		return
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "DROP ROLE IF EXISTS "+name); err != nil {
		t.Logf("drop ephemeral role %s: %v", name, err)
	}
}

func withDatabase(rawURL, database string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.Path = "/" + database
	return parsed.String()
}

func withCredentials(rawURL, user, password, database string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = url.UserPassword(user, password)
	parsed.Path = "/" + database
	return parsed.String()
}

var _ = pgx.ErrNoRows
