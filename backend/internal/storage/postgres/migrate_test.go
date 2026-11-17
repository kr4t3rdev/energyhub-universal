package postgres

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsSortsUpFilesAndIgnoresOthers(t *testing.T) {
	fsys := fstest.MapFS{
		"000002_add_widgets.up.sql":   {Data: []byte("CREATE TABLE widgets (id TEXT);")},
		"000001_create_base.up.sql":   {Data: []byte("CREATE TABLE base (id TEXT);")},
		"000001_create_base.down.sql": {Data: []byte("DROP TABLE base;")},
		"README.md":                   {Data: []byte("not a migration")},
	}

	migrations, err := LoadMigrations(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatalf("migrations = %d, want 2", len(migrations))
	}
	if migrations[0].Version != "000001" || migrations[0].Name != "create_base" {
		t.Fatalf("first migration = %#v", migrations[0])
	}
	if migrations[1].Version != "000002" || migrations[1].Name != "add_widgets" {
		t.Fatalf("second migration = %#v", migrations[1])
	}
	if migrations[0].Statements != "CREATE TABLE base (id TEXT);" {
		t.Fatalf("statements = %q", migrations[0].Statements)
	}
}

func TestLoadMigrationsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		fsys fs.FS
		want error
	}{
		{
			name: "nil filesystem",
			fsys: nil,
			want: ErrNoMigrations,
		},
		{
			name: "empty filesystem",
			fsys: fstest.MapFS{},
			want: ErrNoMigrations,
		},
		{
			name: "invalid filename",
			fsys: fstest.MapFS{
				"1_short.up.sql": {Data: []byte("SELECT 1;")},
			},
			want: ErrInvalidMigration,
		},
		{
			name: "empty statements",
			fsys: fstest.MapFS{
				"000001_empty.up.sql": {Data: []byte("  \n")},
			},
			want: ErrInvalidMigration,
		},
		{
			name: "duplicate version",
			fsys: fstest.MapFS{
				"000001_one.up.sql": {Data: []byte("SELECT 1;")},
				"000001_two.up.sql": {Data: []byte("SELECT 2;")},
			},
			want: ErrDuplicateMigrationVersion,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadMigrations(test.fsys)
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
}

type failingFS struct{ err error }

func (f failingFS) Open(string) (fs.File, error) { return nil, f.err }

type stubDir struct{ entries []fs.DirEntry }

func (d stubDir) Stat() (fs.FileInfo, error) { return nil, errors.New("unused") }
func (d stubDir) Read([]byte) (int, error)   { return 0, io.EOF }
func (d stubDir) Close() error               { return nil }

func (d stubDir) ReadDir(int) ([]fs.DirEntry, error) { return d.entries, nil }

type stubDirEntry struct{ name string }

func (e stubDirEntry) Name() string               { return e.name }
func (e stubDirEntry) IsDir() bool                { return false }
func (e stubDirEntry) Type() fs.FileMode          { return 0 }
func (e stubDirEntry) Info() (fs.FileInfo, error) { return nil, errors.New("unused") }

type unreadableMigrationFS struct{ err error }

func (f unreadableMigrationFS) Open(name string) (fs.File, error) {
	if name == "." {
		return stubDir{entries: []fs.DirEntry{stubDirEntry{name: "000001_one.up.sql"}}}, nil
	}
	return nil, f.err
}

func TestLoadMigrationsPropagatesFilesystemErrors(t *testing.T) {
	globErr := errors.New("filesystem unavailable")
	if _, err := LoadMigrations(failingFS{err: globErr}); !errors.Is(err, globErr) {
		t.Fatalf("glob err = %v, want %v", err, globErr)
	}

	readErr := errors.New("file unavailable")
	if _, err := LoadMigrations(unreadableMigrationFS{err: readErr}); !errors.Is(err, readErr) {
		t.Fatalf("read err = %v, want %v", err, readErr)
	}
}

func migrationPlan() []Migration {
	return []Migration{
		{Version: "000001", Name: "create_base", Statements: "SELECT 1;"},
		{Version: "000002", Name: "add_widgets", Statements: "SELECT 2;"},
	}
}

type fakeMigrationStore struct {
	ensureErr       error
	applied         []string
	appliedErr      error
	applyErr        error
	applyErrVersion string
	ensured         bool
	applyCalls      []string
}

func (f *fakeMigrationStore) EnsureVersionTable(context.Context) error {
	f.ensured = true
	return f.ensureErr
}

func (f *fakeMigrationStore) AppliedVersions(context.Context) ([]string, error) {
	return f.applied, f.appliedErr
}

func (f *fakeMigrationStore) Apply(_ context.Context, migration Migration) error {
	f.applyCalls = append(f.applyCalls, migration.Version)
	if f.applyErr != nil && (f.applyErrVersion == "" || f.applyErrVersion == migration.Version) {
		return f.applyErr
	}
	return nil
}

func TestMigrateAppliesPendingMigrationsAndSkipsAppliedOnes(t *testing.T) {
	store := &fakeMigrationStore{applied: []string{"000001"}}

	result, err := Migrate(context.Background(), store, migrationPlan())
	if err != nil {
		t.Fatal(err)
	}
	if !store.ensured {
		t.Fatal("version table was not ensured")
	}
	if !slices.Equal(store.applyCalls, []string{"000002"}) {
		t.Fatalf("apply calls = %#v, want only 000002", store.applyCalls)
	}
	if !slices.Equal(result.Applied, []string{"000002"}) {
		t.Fatalf("applied = %#v", result.Applied)
	}
	if !slices.Equal(result.Skipped, []string{"000001"}) {
		t.Fatalf("skipped = %#v", result.Skipped)
	}
}

func TestMigrateLeavesFullyAppliedPlanUntouched(t *testing.T) {
	store := &fakeMigrationStore{applied: []string{"000001", "000002"}}

	result, err := Migrate(context.Background(), store, migrationPlan())
	if err != nil {
		t.Fatal(err)
	}
	if len(store.applyCalls) != 0 {
		t.Fatalf("apply calls = %#v, want none", store.applyCalls)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("applied = %#v, want none", result.Applied)
	}
	if !slices.Equal(result.Skipped, []string{"000001", "000002"}) {
		t.Fatalf("skipped = %#v", result.Skipped)
	}
}

func TestMigrateStopsAtFirstFailure(t *testing.T) {
	want := errors.New("migration failed")
	store := &fakeMigrationStore{applyErr: want, applyErrVersion: "000002"}

	result, err := Migrate(context.Background(), store, migrationPlan())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !slices.Equal(store.applyCalls, []string{"000001", "000002"}) {
		t.Fatalf("apply calls = %#v", store.applyCalls)
	}
	if !slices.Equal(result.Applied, []string{"000001"}) {
		t.Fatalf("applied = %#v, want the migration completed before the failure", result.Applied)
	}
}

func TestMigrateRejectsUnknownAppliedMigration(t *testing.T) {
	store := &fakeMigrationStore{applied: []string{"000003"}}

	_, err := Migrate(context.Background(), store, migrationPlan())
	if !errors.Is(err, ErrUnknownAppliedMigration) {
		t.Fatalf("err = %v, want ErrUnknownAppliedMigration", err)
	}
	if len(store.applyCalls) != 0 {
		t.Fatalf("apply calls = %#v, want none", store.applyCalls)
	}
}

func TestMigrateRejectsUnavailableStore(t *testing.T) {
	_, err := Migrate(context.Background(), nil, migrationPlan())
	if !errors.Is(err, ErrMigrationStoreUnavailable) {
		t.Fatalf("err = %v, want ErrMigrationStoreUnavailable", err)
	}
}

func TestMigratePropagatesVersionTableError(t *testing.T) {
	want := errors.New("permission denied")
	store := &fakeMigrationStore{ensureErr: want}

	_, err := Migrate(context.Background(), store, migrationPlan())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if len(store.applyCalls) != 0 {
		t.Fatalf("apply calls = %#v, want none", store.applyCalls)
	}
}

func TestMigratePropagatesAppliedVersionsError(t *testing.T) {
	want := errors.New("query failed")
	store := &fakeMigrationStore{appliedErr: want}

	_, err := Migrate(context.Background(), store, migrationPlan())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if len(store.applyCalls) != 0 {
		t.Fatalf("apply calls = %#v, want none", store.applyCalls)
	}
}

func TestMigrateValidatesPlanBeforeTouchingStore(t *testing.T) {
	tests := []struct {
		name string
		plan []Migration
		want error
	}{
		{
			name: "empty plan",
			plan: nil,
			want: ErrNoMigrations,
		},
		{
			name: "duplicate version",
			plan: []Migration{
				{Version: "000001", Name: "one", Statements: "SELECT 1;"},
				{Version: "000001", Name: "two", Statements: "SELECT 2;"},
			},
			want: ErrDuplicateMigrationVersion,
		},
		{
			name: "descending order",
			plan: []Migration{
				{Version: "000002", Name: "two", Statements: "SELECT 2;"},
				{Version: "000001", Name: "one", Statements: "SELECT 1;"},
			},
			want: ErrUnorderedMigrations,
		},
		{
			name: "invalid version",
			plan: []Migration{
				{Version: "1", Name: "one", Statements: "SELECT 1;"},
			},
			want: ErrInvalidMigration,
		},
		{
			name: "missing name",
			plan: []Migration{
				{Version: "000001", Statements: "SELECT 1;"},
			},
			want: ErrInvalidMigration,
		},
		{
			name: "missing statements",
			plan: []Migration{
				{Version: "000001", Name: "one", Statements: " "},
			},
			want: ErrInvalidMigration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMigrationStore{}
			_, err := Migrate(context.Background(), store, test.plan)
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
			if store.ensured {
				t.Fatal("store was touched before the plan was validated")
			}
		})
	}
}
