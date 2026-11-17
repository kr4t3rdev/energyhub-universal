package postgres

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeMigrationRows struct {
	pgx.Rows
	values  [][1]string
	index   int
	scanErr error
	err     error
	closed  bool
}

func (f *fakeMigrationRows) Next() bool {
	if f.index >= len(f.values) {
		return false
	}
	f.index++
	return true
}

func (f *fakeMigrationRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	*(dest[0].(*string)) = f.values[f.index-1][0]
	return nil
}

func (f *fakeMigrationRows) Err() error {
	return f.err
}

func (f *fakeMigrationRows) Close() {
	f.closed = true
}

type fakeMigrationDB struct {
	execSQL    []string
	execArgs   [][]any
	execErr    error
	rows       pgx.Rows
	queryErr   error
	tx         pgx.Tx
	beginErr   error
	beginCalls int
}

func (f *fakeMigrationDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execSQL = append(f.execSQL, sql)
	f.execArgs = append(f.execArgs, args)
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeMigrationDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return f.rows, f.queryErr
}

func (f *fakeMigrationDB) Begin(_ context.Context) (pgx.Tx, error) {
	f.beginCalls++
	return f.tx, f.beginErr
}

type fakeTx struct {
	pgx.Tx
	execCalls      []string
	execArgs       [][]any
	execErr        error
	failOnCall     int
	commitCalled   bool
	commitErr      error
	rollbackCalled bool
	rollbackErr    error
}

func (f *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls = append(f.execCalls, sql)
	f.execArgs = append(f.execArgs, args)
	if f.failOnCall == len(f.execCalls) {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeTx) Commit(context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rollbackCalled = true
	return f.rollbackErr
}

func TestNewMigrationStoreRejectsMissingDatabase(t *testing.T) {
	store := NewMigrationStore(nil)
	if store == nil {
		t.Fatal("store is nil, want a guard-able value")
	}

	if err := store.EnsureVersionTable(context.Background()); !errors.Is(err, ErrMigrationStoreUnavailable) {
		t.Fatalf("ensure err = %v, want ErrMigrationStoreUnavailable", err)
	}
	if _, err := store.AppliedVersions(context.Background()); !errors.Is(err, ErrMigrationStoreUnavailable) {
		t.Fatalf("applied err = %v, want ErrMigrationStoreUnavailable", err)
	}
	if err := store.Apply(context.Background(), Migration{Version: "000001", Name: "one", Statements: "SELECT 1;"}); !errors.Is(err, ErrMigrationStoreUnavailable) {
		t.Fatalf("apply err = %v, want ErrMigrationStoreUnavailable", err)
	}
}

func TestEnsureVersionTableCreatesTable(t *testing.T) {
	db := &fakeMigrationDB{}

	if err := NewMigrationStore(db).EnsureVersionTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(db.execSQL) != 1 || !strings.Contains(db.execSQL[0], "CREATE TABLE IF NOT EXISTS schema_migrations") {
		t.Fatalf("exec = %#v", db.execSQL)
	}
}

func TestEnsureVersionTablePropagatesError(t *testing.T) {
	want := errors.New("permission denied")
	db := &fakeMigrationDB{execErr: want}

	err := NewMigrationStore(db).EnsureVersionTable(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestAppliedVersionsReturnsOrderedVersions(t *testing.T) {
	rows := &fakeMigrationRows{values: [][1]string{{"000001"}, {"000002"}}}
	db := &fakeMigrationDB{rows: rows}

	versions, err := NewMigrationStore(db).AppliedVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(versions, []string{"000001", "000002"}) {
		t.Fatalf("versions = %#v", versions)
	}
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
}

func TestAppliedVersionsReturnsEmptyList(t *testing.T) {
	db := &fakeMigrationDB{rows: &fakeMigrationRows{}}

	versions, err := NewMigrationStore(db).AppliedVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if versions == nil || len(versions) != 0 {
		t.Fatalf("versions = %#v, want an empty non-nil list", versions)
	}
}

func TestAppliedVersionsPropagatesErrors(t *testing.T) {
	queryErr := errors.New("query failed")
	if _, err := NewMigrationStore(&fakeMigrationDB{queryErr: queryErr}).AppliedVersions(context.Background()); !errors.Is(err, queryErr) {
		t.Fatalf("query err = %v, want %v", err, queryErr)
	}

	scanErr := errors.New("scan failed")
	scanRows := &fakeMigrationRows{values: [][1]string{{"000001"}}, scanErr: scanErr}
	if _, err := NewMigrationStore(&fakeMigrationDB{rows: scanRows}).AppliedVersions(context.Background()); !errors.Is(err, scanErr) {
		t.Fatalf("scan err = %v, want %v", err, scanErr)
	}
	if !scanRows.closed {
		t.Fatal("rows were not closed after a scan error")
	}

	rowsErr := errors.New("result set failed")
	rows := &fakeMigrationRows{err: rowsErr}
	if _, err := NewMigrationStore(&fakeMigrationDB{rows: rows}).AppliedVersions(context.Background()); !errors.Is(err, rowsErr) {
		t.Fatalf("rows err = %v, want %v", err, rowsErr)
	}
	if !rows.closed {
		t.Fatal("rows were not closed after a rows error")
	}
}

func TestApplyRunsMigrationAndRecordsVersionInOneTransaction(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeMigrationDB{tx: tx}
	migration := Migration{Version: "000001", Name: "create_base", Statements: "CREATE TABLE base (id TEXT);"}

	if err := NewMigrationStore(db).Apply(context.Background(), migration); err != nil {
		t.Fatal(err)
	}
	if db.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", db.beginCalls)
	}
	if len(tx.execCalls) != 2 {
		t.Fatalf("exec calls = %#v, want migration SQL and version insert", tx.execCalls)
	}
	if tx.execCalls[0] != migration.Statements {
		t.Fatalf("migration SQL = %q", tx.execCalls[0])
	}
	if len(tx.execArgs[0]) != 1 || tx.execArgs[0][0] != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("migration args = %#v, want simple protocol for multi-statement SQL", tx.execArgs[0])
	}
	if !strings.Contains(tx.execCalls[1], "INSERT INTO schema_migrations") {
		t.Fatalf("record SQL = %q", tx.execCalls[1])
	}
	if !slices.Equal(tx.execArgs[1], []any{"000001", "create_base"}) {
		t.Fatalf("record args = %#v", tx.execArgs[1])
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
	if tx.rollbackCalled {
		t.Fatal("transaction was rolled back after success")
	}
}

func TestApplyRollsBackOnMigrationError(t *testing.T) {
	want := errors.New("bad SQL")
	tx := &fakeTx{execErr: want, failOnCall: 1}

	err := NewMigrationStore(&fakeMigrationDB{tx: tx}).Apply(
		context.Background(),
		Migration{Version: "000001", Name: "create_base", Statements: "BROKEN"},
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if tx.commitCalled {
		t.Fatal("transaction was committed after a failed migration")
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestApplyRollsBackOnVersionRecordError(t *testing.T) {
	want := errors.New("duplicate version")
	tx := &fakeTx{execErr: want, failOnCall: 2}

	err := NewMigrationStore(&fakeMigrationDB{tx: tx}).Apply(
		context.Background(),
		Migration{Version: "000001", Name: "create_base", Statements: "SELECT 1;"},
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if tx.commitCalled {
		t.Fatal("transaction was committed after a failed version record")
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestApplyPropagatesCommitErrorAndRollsBack(t *testing.T) {
	want := errors.New("commit failed")
	tx := &fakeTx{commitErr: want}

	err := NewMigrationStore(&fakeMigrationDB{tx: tx}).Apply(
		context.Background(),
		Migration{Version: "000001", Name: "create_base", Statements: "SELECT 1;"},
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back after a failed commit")
	}
}

func TestApplyPropagatesBeginError(t *testing.T) {
	want := errors.New("no connection")
	tx := &fakeTx{}
	db := &fakeMigrationDB{tx: tx, beginErr: want}

	err := NewMigrationStore(db).Apply(
		context.Background(),
		Migration{Version: "000001", Name: "create_base", Statements: "SELECT 1;"},
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if len(tx.execCalls) != 0 {
		t.Fatalf("exec calls = %#v, want none", tx.execCalls)
	}
}

func TestApplyIgnoresRollbackErrorAfterFailure(t *testing.T) {
	want := errors.New("bad SQL")
	tx := &fakeTx{execErr: want, failOnCall: 1, rollbackErr: errors.New("connection gone")}

	err := NewMigrationStore(&fakeMigrationDB{tx: tx}).Apply(
		context.Background(),
		Migration{Version: "000001", Name: "create_base", Statements: "BROKEN"},
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want the migration failure %v", err, want)
	}
}
