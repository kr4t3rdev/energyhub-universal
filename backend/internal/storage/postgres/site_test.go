package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/site"
)

// fakeSiteRows scans rows of arbitrary columns. A nil cell scans into a
// **string destination as nil, which is how nullable columns behave.
type fakeSiteRows struct {
	rows    [][]any
	index   int
	scanErr error
	err     error
	closed  bool
}

func (f *fakeSiteRows) Next() bool {
	if f.index >= len(f.rows) {
		return false
	}
	f.index++
	return true
}

func (f *fakeSiteRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	row := f.rows[f.index-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan: %d destinations for %d columns", len(dest), len(row))
	}
	for i, d := range dest {
		switch v := d.(type) {
		case *string:
			*v = row[i].(string)
		case **string:
			if row[i] == nil {
				*v = nil
			} else {
				s := row[i].(string)
				*v = &s
			}
		default:
			return fmt.Errorf("scan: unsupported destination %T", d)
		}
	}
	return nil
}

func (f *fakeSiteRows) Err() error {
	return f.err
}

func (f *fakeSiteRows) Close() {
	f.closed = true
}

// fakeSiteTx answers each Query with the next queued result, so a method that
// runs several queries (site, then devices) can be exercised.
type fakeSiteQuery struct {
	rows Rows
	err  error
}

type fakeSiteTx struct {
	execCalls      []string
	execArgs       [][]any
	execErr        error
	queries        []fakeSiteQuery
	queryCalls     int
	querySQLs      []string
	queryArgs      [][]any
	commitCalled   bool
	commitErr      error
	rollbackCalled bool
}

func (f *fakeSiteTx) Exec(_ context.Context, sql string, args ...any) error {
	f.execCalls = append(f.execCalls, sql)
	f.execArgs = append(f.execArgs, args)
	return f.execErr
}

func (f *fakeSiteTx) Query(_ context.Context, sql string, args ...any) (Rows, error) {
	f.queryCalls++
	f.querySQLs = append(f.querySQLs, sql)
	f.queryArgs = append(f.queryArgs, args)
	if f.queryCalls-1 >= len(f.queries) {
		return nil, fmt.Errorf("unexpected query %d: %s", f.queryCalls, sql)
	}
	result := f.queries[f.queryCalls-1]
	return result.rows, result.err
}

func (f *fakeSiteTx) Commit(context.Context) error {
	f.commitCalled = true
	return f.commitErr
}

func (f *fakeSiteTx) Rollback(context.Context) error {
	f.rollbackCalled = true
	return nil
}

type fakeSiteDB struct {
	tx       *fakeSiteTx
	beginErr error
	begins   int
}

func (f *fakeSiteDB) Begin(context.Context) (Tx, error) {
	f.begins++
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

func siteRows(cells ...[]any) *fakeSiteRows {
	return &fakeSiteRows{rows: cells}
}

func TestSiteReaderListsSitesForVerifiedSubject(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{
		{rows: siteRows(
			[]any{"11111111-1111-4111-8111-111111111111", "Casa de ejemplo", "Europe/Madrid"},
		)},
	}}
	db := &fakeSiteDB{tx: tx}

	sites, err := NewSiteReader(db).ListSites(context.Background(), " dev|user-1 ", " org-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].ID != "11111111-1111-4111-8111-111111111111" || sites[0].Name != "Casa de ejemplo" || sites[0].Timezone != "Europe/Madrid" {
		t.Fatalf("sites = %#v, want the single site", sites)
	}
	if len(tx.execCalls) != 1 || tx.execCalls[0] != setMembershipSubjectQuery {
		t.Fatalf("exec calls = %#v, want only the subject setting", tx.execCalls)
	}
	if len(tx.execArgs[0]) != 1 || tx.execArgs[0][0] != "dev|user-1" {
		t.Fatalf("subject arg = %#v, want the trimmed subject", tx.execArgs[0])
	}
	if len(tx.queryArgs[0]) != 1 || tx.queryArgs[0][0] != "org-1" {
		t.Fatalf("query args = %#v, want the trimmed organization", tx.queryArgs[0])
	}
	if !tx.commitCalled || tx.rollbackCalled {
		t.Fatal("transaction was not committed cleanly")
	}
}

func TestSiteReaderListsEmptySitesAsEmptySlice(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{{rows: siteRows()}}}
	db := &fakeSiteDB{tx: tx}

	sites, err := NewSiteReader(db).ListSites(context.Background(), "dev|user-1", "org-1")
	if err != nil {
		t.Fatal(err)
	}
	if sites == nil || len(sites) != 0 {
		t.Fatalf("sites = %#v, want an empty non-nil slice", sites)
	}
}

func TestSiteReaderRejectsUnauthenticatedSubjectBeforeBegin(t *testing.T) {
	db := &fakeSiteDB{tx: &fakeSiteTx{}}

	_, err := NewSiteReader(db).ListSites(context.Background(), " ", "org-1")
	if !errors.Is(err, authz.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestSiteReaderRejectsEmptyOrganizationBeforeBegin(t *testing.T) {
	db := &fakeSiteDB{tx: &fakeSiteTx{}}

	_, err := NewSiteReader(db).ListSites(context.Background(), "dev|user-1", " ")
	if !errors.Is(err, site.ErrInvalidScope) {
		t.Fatalf("err = %v, want ErrInvalidScope", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestSiteReaderRollsBackOnQueryError(t *testing.T) {
	want := errors.New("database unavailable")
	tx := &fakeSiteTx{queries: []fakeSiteQuery{{rows: nil, err: want}}}
	db := &fakeSiteDB{tx: tx}

	_, err := NewSiteReader(db).ListSites(context.Background(), "dev|user-1", "org-1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled || tx.commitCalled {
		t.Fatal("failed query did not roll back")
	}
}

func TestSiteReaderOverviewReturnsSiteDevicesAndNoReadings(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{
		{rows: siteRows([]any{"11111111-1111-4111-8111-111111111111", "Casa de ejemplo", "Europe/Madrid"})},
		{rows: siteRows(
			[]any{"22222222-2222-4222-8222-222222222222", "Inversor solar", "inversor", "En línea"},
			[]any{"33333333-3333-4333-8333-333333333333", "Batería doméstica", "batería", nil},
		)},
	}}
	db := &fakeSiteDB{tx: tx}

	overview, err := NewSiteReader(db).GetOverview(context.Background(), "dev|user-1", "org-1", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if overview.Site.ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("site = %#v, want the site", overview.Site)
	}
	if overview.Readings == nil || len(overview.Readings) != 0 {
		t.Fatalf("readings = %#v, want an empty non-nil slice: there is no telemetry yet", overview.Readings)
	}
	if overview.UpdatedAt != nil {
		t.Fatalf("updatedAt = %v, want nil: there is no telemetry yet", overview.UpdatedAt)
	}
	if len(overview.Devices) != 2 {
		t.Fatalf("devices = %#v, want two devices", overview.Devices)
	}
	if overview.Devices[0].Status == nil || *overview.Devices[0].Status != "En línea" {
		t.Fatalf("device status = %#v, want En línea", overview.Devices[0].Status)
	}
	if overview.Devices[1].Status != nil {
		t.Fatalf("device status = %#v, want nil", overview.Devices[1].Status)
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
}

func TestSiteReaderOverviewReturnsNotFoundForInvisibleSite(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{{rows: siteRows()}}}
	db := &fakeSiteDB{tx: tx}

	_, err := NewSiteReader(db).GetOverview(context.Background(), "dev|user-1", "org-1", "99999999-9999-4999-8999-999999999999")
	if !errors.Is(err, site.ErrSiteNotFound) {
		t.Fatalf("err = %v, want ErrSiteNotFound", err)
	}
	if tx.queryCalls != 1 {
		t.Fatalf("query calls = %d, want 1: the devices must not be read for an invisible site", tx.queryCalls)
	}
	if !tx.rollbackCalled || tx.commitCalled {
		t.Fatal("not-found overview did not roll back")
	}
}

func TestSiteReaderOverviewTreatsMalformedSiteIDAsNotFound(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{{rows: siteRows()}}}
	db := &fakeSiteDB{tx: tx}

	_, err := NewSiteReader(db).GetOverview(context.Background(), "dev|user-1", "org-1", "not-a-uuid")
	if !errors.Is(err, site.ErrSiteNotFound) {
		t.Fatalf("err = %v, want ErrSiteNotFound, never a database error", err)
	}
}

func TestSiteReaderListDevicesChecksSiteVisibilityFirst(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{
		{rows: siteRows([]any{"11111111-1111-4111-8111-111111111111", "Casa de ejemplo", "Europe/Madrid"})},
		{rows: siteRows([]any{"22222222-2222-4222-8222-222222222222", "Inversor solar", "inversor", nil})},
	}}
	db := &fakeSiteDB{tx: tx}

	devices, err := NewSiteReader(db).ListDevices(context.Background(), "dev|user-1", "org-1", "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Kind != "inversor" || devices[0].Status != nil {
		t.Fatalf("devices = %#v, want the single device with nil status", devices)
	}
}

func TestSiteReaderListDevicesReturnsNotFoundForInvisibleSite(t *testing.T) {
	tx := &fakeSiteTx{queries: []fakeSiteQuery{{rows: siteRows()}}}
	db := &fakeSiteDB{tx: tx}

	_, err := NewSiteReader(db).ListDevices(context.Background(), "dev|user-1", "org-1", "99999999-9999-4999-8999-999999999999")
	if !errors.Is(err, site.ErrSiteNotFound) {
		t.Fatalf("err = %v, want ErrSiteNotFound", err)
	}
}

func TestSiteReaderRejectsEmptySiteIDBeforeBegin(t *testing.T) {
	db := &fakeSiteDB{tx: &fakeSiteTx{}}

	_, err := NewSiteReader(db).GetOverview(context.Background(), "dev|user-1", "org-1", " ")
	if !errors.Is(err, site.ErrInvalidSiteID) {
		t.Fatalf("err = %v, want ErrInvalidSiteID", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestSiteReaderFailsClosedWithoutDatabase(t *testing.T) {
	_, err := NewSiteReader(nil).ListSites(context.Background(), "dev|user-1", "org-1")
	if !errors.Is(err, site.ErrReaderUnavailable) {
		t.Fatalf("err = %v, want ErrReaderUnavailable", err)
	}
}
