package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/storage/postgres"
	"energyhub-universal/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// ErrRuntimeCredentialForMigrations is returned when the migration connection
// uses the read only runtime role. That credential must never apply
// migrations: it would own the tables and could disable row level security.
var ErrRuntimeCredentialForMigrations = errors.New("the runtime credential cannot apply migrations")

// runtimeRoleName is the role the API uses to read memberships.
const runtimeRoleName = "energyhub_runtime"

// migrationDatabaseURL resolves the privileged connection used to apply
// migrations. The runtime credential of the API is never accepted here.
func migrationDatabaseURL(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	return strings.TrimSpace(getenv("MIGRATION_DATABASE_URL"))
}

// validateMigrationCredential refuses a connection that would leave the
// database owned by the credential the API reads with.
func validateMigrationCredential(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("parse migration database URL: %w", err)
	}
	if parsed.User == nil {
		return nil
	}
	if parsed.User.Username() == runtimeRoleName {
		return ErrRuntimeCredentialForMigrations
	}
	return nil
}

func run() error {
	databaseURL := migrationDatabaseURL(os.Getenv)
	if databaseURL == "" {
		return errors.New("MIGRATION_DATABASE_URL is required")
	}
	if err := validateMigrationCredential(databaseURL); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	plan, err := postgres.LoadMigrations(migrations.Files)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	result, err := postgres.Migrate(ctx, postgres.NewMigrationStore(pool), plan)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Printf("migrations complete: applied=%d skipped=%d", len(result.Applied), len(result.Skipped))
	return nil
}
