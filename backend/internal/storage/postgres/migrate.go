package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrNoMigrations              = errors.New("no migrations found")
	ErrInvalidMigration          = errors.New("invalid migration")
	ErrDuplicateMigrationVersion = errors.New("duplicate migration version")
	ErrUnorderedMigrations       = errors.New("migrations are not in ascending order")
	ErrUnknownAppliedMigration   = errors.New("database has an applied migration unknown to this binary")
	ErrMigrationStoreUnavailable = errors.New("migration store unavailable")
)

var (
	migrationFilenamePattern = regexp.MustCompile(`^([0-9]{6})_([a-z0-9_]+)\.up\.sql$`)
	migrationVersionPattern  = regexp.MustCompile(`^[0-9]{6}$`)
)

// Migration is one versioned, forward-only schema change.
type Migration struct {
	Version    string
	Name       string
	Statements string
}

// MigrationResult records what a run did, in plan order.
type MigrationResult struct {
	Applied []string
	Skipped []string
}

// MigrationStore persists schema versions and applies migrations. Apply must
// run the migration SQL and the version record in a single transaction.
type MigrationStore interface {
	EnsureVersionTable(ctx context.Context) error
	AppliedVersions(ctx context.Context) ([]string, error)
	Apply(ctx context.Context, migration Migration) error
}

// LoadMigrations reads NNNNNN_name.up.sql files from fsys, validates their
// names and bodies, and returns them in ascending version order. Down
// migrations and documentation files are ignored.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	if fsys == nil {
		return nil, ErrNoMigrations
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return nil, ErrNoMigrations
	}

	migrations := make([]Migration, 0, len(names))
	seen := make(map[string]string, len(names))
	for _, name := range names {
		matches := migrationFilenamePattern.FindStringSubmatch(name)
		if matches == nil {
			return nil, fmt.Errorf("%w: filename %q must match NNNNNN_name.up.sql", ErrInvalidMigration, name)
		}
		version, label := matches[1], matches[2]
		if previous, ok := seen[version]; ok {
			return nil, fmt.Errorf("%w: %s and %s", ErrDuplicateMigrationVersion, previous, name)
		}
		seen[version] = name

		statements, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		if strings.TrimSpace(string(statements)) == "" {
			return nil, fmt.Errorf("%w: migration %s has no statements", ErrInvalidMigration, name)
		}
		migrations = append(migrations, Migration{
			Version:    version,
			Name:       label,
			Statements: string(statements),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

// Migrate applies every pending migration in order. It refuses plans that are
// not strictly ordered and databases that already have a version this binary
// does not know, so an older binary cannot silently run against a newer schema.
// Concurrent runs are not coordinated: the schema_migrations primary key makes
// a duplicate run fail instead of applying a version twice, so run this as a
// single deployment job.
func Migrate(ctx context.Context, store MigrationStore, plan []Migration) (MigrationResult, error) {
	result := MigrationResult{Applied: []string{}, Skipped: []string{}}
	if err := validatePlan(plan); err != nil {
		return result, err
	}
	if store == nil {
		return result, ErrMigrationStoreUnavailable
	}

	if err := store.EnsureVersionTable(ctx); err != nil {
		return result, fmt.Errorf("ensure schema_migrations: %w", err)
	}
	applied, err := store.AppliedVersions(ctx)
	if err != nil {
		return result, fmt.Errorf("read applied migrations: %w", err)
	}

	known := make(map[string]bool, len(plan))
	for _, migration := range plan {
		known[migration.Version] = true
	}
	appliedSet := make(map[string]bool, len(applied))
	for _, version := range applied {
		if !known[version] {
			return result, fmt.Errorf("%w: version %s", ErrUnknownAppliedMigration, version)
		}
		appliedSet[version] = true
	}

	for _, migration := range plan {
		if appliedSet[migration.Version] {
			result.Skipped = append(result.Skipped, migration.Version)
			continue
		}
		if err := store.Apply(ctx, migration); err != nil {
			return result, fmt.Errorf("apply migration %s (%s): %w", migration.Version, migration.Name, err)
		}
		result.Applied = append(result.Applied, migration.Version)
	}
	return result, nil
}

func validatePlan(plan []Migration) error {
	if len(plan) == 0 {
		return ErrNoMigrations
	}

	seen := make(map[string]bool, len(plan))
	previous := ""
	for _, migration := range plan {
		if !migrationVersionPattern.MatchString(migration.Version) {
			return fmt.Errorf("%w: version %q must be six digits", ErrInvalidMigration, migration.Version)
		}
		if strings.TrimSpace(migration.Name) == "" {
			return fmt.Errorf("%w: version %s has no name", ErrInvalidMigration, migration.Version)
		}
		if strings.TrimSpace(migration.Statements) == "" {
			return fmt.Errorf("%w: version %s has no statements", ErrInvalidMigration, migration.Version)
		}
		if seen[migration.Version] {
			return fmt.Errorf("%w: version %s", ErrDuplicateMigrationVersion, migration.Version)
		}
		seen[migration.Version] = true
		if previous != "" && migration.Version < previous {
			return fmt.Errorf("%w: %s before %s", ErrUnorderedMigrations, migration.Version, previous)
		}
		previous = migration.Version
	}
	return nil
}
