package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

// The production adapter must keep accepting a pgx pool as pgx evolves.
var _ pgxQuerier = (*pgxpool.Pool)(nil)

type fakeRows struct {
	rows     [][3]string
	index    int
	scanErr  error
	err      error
	closed   bool
	nextCall int
}

func (f *fakeRows) Next() bool {
	f.nextCall++
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

type fakeQuerier struct {
	sql   string
	args  []any
	rows  *fakeRows
	err   error
	calls int
}

func (f *fakeQuerier) Query(_ context.Context, sql string, args ...any) (Rows, error) {
	f.calls++
	f.sql = sql
	f.args = args
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func TestMembershipReaderListsOrganizationsByVerifiedSubject(t *testing.T) {
	rows := &fakeRows{rows: [][3]string{{"org-1", "Demo", "viewer"}}}
	db := &fakeQuerier{rows: rows}

	organizations, err := NewMembershipReader(db).ListOrganizations(context.Background(), " auth0|user-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if db.calls != 1 || db.sql != listOrganizationsQuery {
		t.Fatalf("query calls = %d, sql = %q", db.calls, db.sql)
	}
	if len(db.args) != 1 || db.args[0] != "auth0|user-1" {
		t.Fatalf("args = %#v, want the trimmed verified subject", db.args)
	}
	want := contracts.OrganizationSummary{ID: "org-1", Name: "Demo", Role: contracts.RoleViewer}
	if len(organizations) != 1 || organizations[0] != want {
		t.Fatalf("organizations = %#v, want %#v", organizations, want)
	}
	if !rows.closed {
		t.Fatal("rows were not closed")
	}
}

func TestMembershipReaderReturnsEmptyListWithoutRows(t *testing.T) {
	rows := &fakeRows{}
	organizations, err := NewMembershipReader(&fakeQuerier{rows: rows}).
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
}

func TestMembershipReaderPropagatesQueryError(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := NewMembershipReader(&fakeQuerier{err: want}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMembershipReaderPropagatesScanError(t *testing.T) {
	want := errors.New("scan failed")
	rows := &fakeRows{rows: [][3]string{{"org-1", "Demo", "viewer"}}, scanErr: want}

	_, err := NewMembershipReader(&fakeQuerier{rows: rows}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !rows.closed {
		t.Fatal("rows were not closed after scan error")
	}
}

func TestMembershipReaderPropagatesRowsError(t *testing.T) {
	want := errors.New("result set failed")
	rows := &fakeRows{
		rows: [][3]string{{"org-1", "Demo", "viewer"}},
		err:  want,
	}

	_, err := NewMembershipReader(&fakeQuerier{rows: rows}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMembershipReaderRejectsInvalidRoleFromDatabase(t *testing.T) {
	rows := &fakeRows{rows: [][3]string{{"org-1", "Demo", "superuser"}}}

	_, err := NewMembershipReader(&fakeQuerier{rows: rows}).
		ListOrganizations(context.Background(), "auth0|user-1")
	if !errors.Is(err, ErrInvalidMembershipRole) {
		t.Fatalf("err = %v, want ErrInvalidMembershipRole", err)
	}
}

func TestMembershipReaderRejectsMissingSubjectBeforeQuery(t *testing.T) {
	db := &fakeQuerier{rows: &fakeRows{}}

	_, err := NewMembershipReader(db).ListOrganizations(context.Background(), " ")
	if !errors.Is(err, authz.ErrUnauthenticated) {
		t.Fatalf("err = %v, want authz.ErrUnauthenticated", err)
	}
	if db.calls != 0 {
		t.Fatalf("query calls = %d, want 0", db.calls)
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

func TestNewQueryerRejectsNilQuerier(t *testing.T) {
	if NewQueryer(nil) != nil {
		t.Fatal("adapter from nil querier is not nil")
	}
}

type fakePgxQuerier struct {
	rows  pgx.Rows
	err   error
	sql   string
	args  []any
	calls int
}

func (f *fakePgxQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.calls++
	f.sql = sql
	f.args = args
	return f.rows, f.err
}

func TestPgxAdapterForwardsQueryAndArguments(t *testing.T) {
	db := &fakePgxQuerier{}

	rows, err := NewQueryer(db).Query(context.Background(), "SELECT 1", "auth0|user-1")
	if err != nil {
		t.Fatal(err)
	}
	if rows != nil {
		t.Fatalf("rows = %#v, want nil from fake", rows)
	}
	if db.calls != 1 || db.sql != "SELECT 1" || len(db.args) != 1 || db.args[0] != "auth0|user-1" {
		t.Fatalf("query = %d %q %#v", db.calls, db.sql, db.args)
	}
}

func TestPgxAdapterPropagatesQueryError(t *testing.T) {
	want := errors.New("pool exhausted")
	_, err := NewQueryer(&fakePgxQuerier{err: want}).Query(context.Background(), "SELECT 1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
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
