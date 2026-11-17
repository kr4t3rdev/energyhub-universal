package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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

// migrationDatabaseURL resolves the privileged connection used to apply
// migrations. MIGRATION_DATABASE_URL wins; DATABASE_URL is the fallback for
// single-credential environments.
func migrationDatabaseURL(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	if url := strings.TrimSpace(getenv("MIGRATION_DATABASE_URL")); url != "" {
		return url
	}
	return strings.TrimSpace(getenv("DATABASE_URL"))
}

func run() error {
	databaseURL := migrationDatabaseURL(os.Getenv)
	if databaseURL == "" {
		return errors.New("MIGRATION_DATABASE_URL is required")
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
