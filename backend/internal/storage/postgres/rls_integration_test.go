package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/migrations"
)

// The PostgreSQL integration test proves the tenant boundary against a real
// server. It is skipped unless TEST_DATABASE_URL points to a PostgreSQL
// superuser, because the test creates roles and a throwaway database.
//
//	TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55432/postgres' go test ./internal/storage/postgres
const integrationSuperuserURLEnv = "TEST_DATABASE_URL"

const (
	integrationRuntimeRole = "energyhub_runtime"
	integrationAdminRole   = "energyhub_admin"
	integrationOwnerRole   = "energyhub_owner"
	integrationAdminGroup  = "energyhub_membership_admin"
	integrationPassword    = "dev-integration"
)

type integrationEnvironment struct {
	runtimeURL string
	adminURL   string
}

func TestIntegrationTenantIsolationAndAdminWrites(t *testing.T) {
	superuserURL := strings.TrimSpace(os.Getenv(integrationSuperuserURLEnv))
	if superuserURL == "" {
		t.Skipf("%s is not set; skipping PostgreSQL integration test", integrationSuperuserURLEnv)
	}

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

		subjects := readSubjects(t, runtimePool, "dev|user-a")
		if len(subjects) != 1 || subjects[0] != "dev|user-a" {
			t.Fatalf("subjects = %#v, want [dev|user-a]", subjects)
		}

		others := readSubjects(t, runtimePool, "dev|user-b")
		if len(others) != 0 {
			t.Fatalf("subjects = %#v, want none for another subject", others)
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

	t.Run("admin reads every membership through the role boundary", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		rows, err := adminPool.Query(ctx, "SELECT subject FROM public.organization_memberships")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		subjects := []string{}
		for rows.Next() {
			var subject string
			if err := rows.Scan(&subject); err != nil {
				t.Fatal(err)
			}
			subjects = append(subjects, subject)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(subjects) != 1 || subjects[0] != "dev|user-a" {
			t.Fatalf("subjects = %#v, want [dev|user-a]", subjects)
		}
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
}

func readSubjects(t *testing.T, pool *pgxpool.Pool, subject string) []string {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "SELECT set_config('app.current_subject', $1, false)", subject); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, "SELECT subject FROM public.organization_memberships")
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

// newIntegrationEnvironment creates the shared roles, a throwaway database and
// applies every migration to it.
func newIntegrationEnvironment(t *testing.T, superuserURL string) integrationEnvironment {
	t.Helper()
	ctx := context.Background()

	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	ensureIntegrationRoles(t, ctx, superuserPool)

	databaseName := fmt.Sprintf("energyhub_it_%d", os.Getpid())
	if _, err := superuserPool.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseName); err != nil {
		t.Fatal(err)
	}
	if _, err := superuserPool.Exec(ctx, "CREATE DATABASE "+databaseName); err != nil {
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

	databaseURL := withDatabase(superuserURL, databaseName)
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
		runtimeURL: withCredentials(superuserURL, integrationRuntimeRole, integrationPassword, databaseName),
		adminURL:   withCredentials(superuserURL, integrationAdminRole, integrationPassword, databaseName),
	}
}

func ensureIntegrationRoles(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	statements := []string{
		`DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + integrationAdminGroup + `') THEN
				CREATE ROLE ` + integrationAdminGroup + ` NOLOGIN;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + integrationOwnerRole + `') THEN
				CREATE ROLE ` + integrationOwnerRole + ` NOLOGIN BYPASSRLS;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + integrationRuntimeRole + `') THEN
				CREATE ROLE ` + integrationRuntimeRole + ` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + integrationAdminRole + `') THEN
				CREATE ROLE ` + integrationAdminRole + ` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
			END IF;
		END
		$$;`,
		"ALTER ROLE " + integrationRuntimeRole + " PASSWORD '" + integrationPassword + "'",
		"ALTER ROLE " + integrationAdminRole + " PASSWORD '" + integrationPassword + "'",
		"ALTER ROLE " + integrationOwnerRole + " BYPASSRLS",
		"GRANT " + integrationAdminGroup + " TO " + integrationAdminRole,
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("statement %q: %v", statement, err)
		}
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
