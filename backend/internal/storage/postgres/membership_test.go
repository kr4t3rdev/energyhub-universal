package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

type fakeRows struct {
	rows    [][3]string
	index   int
	scanErr error
	err     error
	closed  bool
}

func (f *fakeRows) Next() bool {
	if f.index >= len(f.rows) {
		return false
	}
	f.index++
	return true
}

func (f *fakeRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	row := f.rows[f.index-1]
	*(dest[0].(*string)) = row[0]
	*(dest[1].(*string)) = row[1]
	*(dest[2].(*string)) = row[2]
	return nil
}

func (f *fakeRows) Err() error {
	return f.err
}

func (f *fakeRows) Close() {
	f.closed = true
}

type fakeMembershipTx struct {
	execCalls      []string
	execArgs       [][]any
	execErr        error
	failOnExec     int
	rows           *fakeRows
	queryErr       error
	queryCalls     int
	querySQL       string
	queryArgs      []any
	commitCalled   bool
	commitErr      error
	rollbackCalled bool
	rollbackErr    error
}

func (f *fakeMembershipTx) Exec(_ context.Context, sql string, args ...any) error {
	f.execCalls = append(f.execCalls, sql)
	f.execArgs = append(f.execArgs, args)
	if f.failOnExec == len(f.execCalls) {
		return f.execErr
	}
	return nil
}

func (f *fakeMembershipTx) Query(_ context.Context, sql string, args ...any) (Rows, error) {
	f.queryCalls++
	f.querySQL = sql
	f.queryArgs = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.rows, nil
}

func (f *fakeMembershipTx) Commit(context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeMembershipTx) Rollback(context.Context) error {
	f.rollbackCalled = true
	return f.rollbackErr
}

type fakeDB struct {
	tx       *fakeMembershipTx
	beginErr error
	begins   int
}

func (f *fakeDB) Begin(context.Context) (Tx, error) {
	f.begins++
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

func TestMembershipReaderListsOrganizationsByVerifiedSubject(t *testing.T) {
	rows := &fakeRows{rows: [][3]string{{"org-1", "Demo", "viewer"}}}
	tx := &fakeMembershipTx{rows: rows}
	db := &fakeDB{tx: tx}

	organizations, err := NewMembershipReader(db).ListOrganizations(context.Background(), " auth0|user-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if db.begins != 1 {
		t.Fatalf("begin calls = %d, want 1", db.begins)
	}
	if len(tx.execCalls) != 1 || tx.execCalls[0] != setMembershipSubjectQuery {
		t.Fatalf("exec calls = %#v, want the RLS subject setting", tx.execCalls)
	}
	if len(tx.execArgs[0]) != 1 || tx.execArgs[0][0] != "auth0|user-1" {
		t.Fatalf("subject args = %#v, want the trimmed verified subject", tx.execArgs[0])
	}
	if tx.queryCalls != 1 || tx.querySQL != listOrganizationsQuery {
		t.Fatalf("query calls = %d, sql = %q", tx.queryCalls, tx.querySQL)
	}
	if len(tx.queryArgs) != 1 || tx.queryArgs[0] != "auth0|user-1" {
		t.Fatalf("query args = %#v, want the trimmed verified subject", tx.queryArgs)
	}
	want := contracts.OrganizationSummary{ID: "org-1", Name: "Demo", Role: contracts.RoleViewer}
	if len(organizations) != 1 || organizations[0] != want {
		t.Fatalf("organizations = %#v, want %#v", organizations, want)
	}
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
	if tx.rollbackCalled {
		t.Fatal("transaction was rolled back after success")
	}
}

func TestMembershipReaderReturnsEmptyListWithoutRows(t *testing.T) {
	rows := &fakeRows{}
	tx := &fakeMembershipTx{rows: rows}

	organizations, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if err != nil {
		t.Fatal(err)
	}
	if organizations == nil || len(organizations) != 0 {
		t.Fatalf("organizations = %#v, want empty non-nil list", organizations)
	}
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
}

func TestMembershipReaderPropagatesBeginError(t *testing.T) {
	want := errors.New("pool exhausted")
	_, err := NewMembershipReader(&fakeDB{beginErr: want}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMembershipReaderPropagatesSubjectSettingError(t *testing.T) {
	want := errors.New("set_config failed")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 1, rows: &fakeRows{}}

	_, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if tx.queryCalls != 0 {
		t.Fatalf("query calls = %d, want 0", tx.queryCalls)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipReaderPropagatesQueryError(t *testing.T) {
	want := errors.New("database unavailable")
	tx := &fakeMembershipTx{queryErr: want}

	_, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipReaderPropagatesScanError(t *testing.T) {
	want := errors.New("scan failed")
	rows := &fakeRows{rows: [][3]string{{"org-1", "Demo", "viewer"}}, scanErr: want}
	tx := &fakeMembershipTx{rows: rows}

	_, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !rows.closed {
		t.Fatal("rows were not closed after scan error")
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipReaderPropagatesRowsError(t *testing.T) {
	want := errors.New("result set failed")
	rows := &fakeRows{
		rows: [][3]string{{"org-1", "Demo", "viewer"}},
		err:  want,
	}
	tx := &fakeMembershipTx{rows: rows}

	_, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !rows.closed {
		t.Fatal("rows were not closed after a rows error")
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipReaderRejectsInvalidRoleFromDatabase(t *testing.T) {
	rows := &fakeRows{rows: [][3]string{{"org-1", "Demo", "superuser"}}}
	tx := &fakeMembershipTx{rows: rows}

	_, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, ErrInvalidMembershipRole) {
		t.Fatalf("err = %v, want ErrInvalidMembershipRole", err)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipReaderPropagatesCommitError(t *testing.T) {
	want := errors.New("commit failed")
	tx := &fakeMembershipTx{rows: &fakeRows{}, commitErr: want}

	_, err := NewMembershipReader(&fakeDB{tx: tx}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back after a failed commit")
	}
}

func TestMembershipReaderRejectsMissingSubjectBeforeTransaction(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{rows: &fakeRows{}}}

	_, err := NewMembershipReader(db).ListOrganizations(context.Background(), " ")
	if !errors.Is(err, authz.ErrUnauthenticated) {
		t.Fatalf("err = %v, want authz.ErrUnauthenticated", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestMembershipReaderRejectsUnavailableDatabase(t *testing.T) {
	_, err := NewMembershipReader(nil).ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, authz.ErrMembershipReaderUnavailable) {
		t.Fatalf("err = %v, want authz.ErrMembershipReaderUnavailable", err)
	}
}

func TestMembershipReaderRejectsNilReceiver(t *testing.T) {
	var reader *MembershipReader
	_, err := reader.ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, authz.ErrMembershipReaderUnavailable) {
		t.Fatalf("err = %v, want authz.ErrMembershipReaderUnavailable", err)
	}
}

func TestNewDBRejectsNilBeginner(t *testing.T) {
	if NewDB(nil) != nil {
		t.Fatal("adapter from nil beginner is not nil")
	}
}

type fakePgxBeginner struct {
	tx    pgx.Tx
	err   error
	calls int
}

func (f *fakePgxBeginner) Begin(context.Context) (pgx.Tx, error) {
	f.calls++
	return f.tx, f.err
}

type fakePgxTx struct {
	pgx.Tx
	execSQL        string
	execArgs       []any
	execErr        error
	rows           pgx.Rows
	queryErr       error
	commitCalled   bool
	commitErr      error
	rollbackCalled bool
	rollbackErr    error
}

func (f *fakePgxTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execSQL = sql
	f.execArgs = args
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakePgxTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return f.rows, f.queryErr
}

func (f *fakePgxTx) Commit(context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakePgxTx) Rollback(context.Context) error {
	f.rollbackCalled = true
	return f.rollbackErr
}

// fakePgxRows only exists to satisfy pgx.Rows in the adapter test; the narrow
// Rows port is exercised by fakeRows above.
type fakePgxRows struct {
	pgx.Rows
}

func TestDBBeginWrapsPgxTransaction(t *testing.T) {
	rows := &fakePgxRows{}
	pgxTransaction := &fakePgxTx{rows: rows}
	beginner := &fakePgxBeginner{tx: pgxTransaction}

	tx, err := NewDB(beginner).Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if beginner.calls != 1 {
		t.Fatalf("begin calls = %d, want 1", beginner.calls)
	}
	if err := tx.Exec(context.Background(), "SELECT 1", "arg"); err != nil {
		t.Fatal(err)
	}
	if pgxTransaction.execSQL != "SELECT 1" || len(pgxTransaction.execArgs) != 1 || pgxTransaction.execArgs[0] != "arg" {
		t.Fatalf("exec = %q %#v", pgxTransaction.execSQL, pgxTransaction.execArgs)
	}
	gotRows, err := tx.Query(context.Background(), "SELECT 2")
	if err != nil {
		t.Fatal(err)
	}
	if gotRows != any(rows) {
		t.Fatalf("rows = %#v, want the pgx rows", gotRows)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !pgxTransaction.commitCalled || !pgxTransaction.rollbackCalled {
		t.Fatalf("commit = %v, rollback = %v", pgxTransaction.commitCalled, pgxTransaction.rollbackCalled)
	}
}

func TestDBBeginPropagatesError(t *testing.T) {
	want := errors.New("no connection")
	_, err := NewDB(&fakePgxBeginner{err: want}).Begin(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestPgxTxPropagatesErrors(t *testing.T) {
	execErr := errors.New("exec failed")
	tx := pgxTx{tx: &fakePgxTx{execErr: execErr}}
	if err := tx.Exec(context.Background(), "SELECT 1"); !errors.Is(err, execErr) {
		t.Fatalf("exec err = %v, want %v", err, execErr)
	}

	queryErr := errors.New("query failed")
	tx = pgxTx{tx: &fakePgxTx{queryErr: queryErr}}
	if _, err := tx.Query(context.Background(), "SELECT 1"); !errors.Is(err, queryErr) {
		t.Fatalf("query err = %v, want %v", err, queryErr)
	}

	commitErr := errors.New("commit failed")
	tx = pgxTx{tx: &fakePgxTx{commitErr: commitErr}}
	if err := tx.Commit(context.Background()); !errors.Is(err, commitErr) {
		t.Fatalf("commit err = %v, want %v", err, commitErr)
	}

	rollbackErr := errors.New("rollback failed")
	tx = pgxTx{tx: &fakePgxTx{rollbackErr: rollbackErr}}
	if err := tx.Rollback(context.Background()); !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback err = %v, want %v", err, rollbackErr)
	}
}

func TestMembershipRoleAcceptsEveryContractRole(t *testing.T) {
	for _, role := range []contracts.OrganizationRole{
		contracts.RoleOwner,
		contracts.RoleAdmin,
		contracts.RoleMember,
		contracts.RoleViewer,
	} {
		got, err := membershipRole(string(role))
		if err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		if got != role {
			t.Fatalf("role = %q, want %q", got, role)
		}
	}
}
