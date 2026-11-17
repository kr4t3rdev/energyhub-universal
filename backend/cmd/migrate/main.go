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

func run() error {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
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
