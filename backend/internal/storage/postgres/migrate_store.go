package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const createSchemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

const selectAppliedMigrationsQuery = `
SELECT version
FROM schema_migrations
ORDER BY version`

const recordMigrationQuery = `
INSERT INTO schema_migrations (version, name)
VALUES ($1, $2)`

// MigrationDB is the database surface needed by the migration runner.
type MigrationDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

var _ MigrationDB = (*pgxpool.Pool)(nil)

// PgxMigrationStore applies migrations atomically. Migration SQL is sent with
// the simple protocol because a single migration file may contain several
// statements, which the extended protocol rejects.
type PgxMigrationStore struct {
	db MigrationDB
}

func NewMigrationStore(db MigrationDB) *PgxMigrationStore {
	return &PgxMigrationStore{db: db}
}

func (s *PgxMigrationStore) EnsureVersionTable(ctx context.Context) error {
	if s == nil || s.db == nil {
		return ErrMigrationStoreUnavailable
	}
	if _, err := s.db.Exec(ctx, createSchemaMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func (s *PgxMigrationStore) AppliedVersions(ctx context.Context) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, ErrMigrationStoreUnavailable
	}
	rows, err := s.db.Query(ctx, selectAppliedMigrationsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make([]string, 0)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s *PgxMigrationStore) Apply(ctx context.Context, migration Migration) error {
	if s == nil || s.db == nil {
		return ErrMigrationStoreUnavailable
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err := tx.Exec(ctx, migration.Statements, pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("execute migration %s: %w", migration.Version, err)
	}
	if _, err := tx.Exec(ctx, recordMigrationQuery, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.Version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.Version, err)
	}
	committed = true
	return nil
}
