package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRoleRow struct {
	values []any
	err    error
}

func (r fakeRoleRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, destination := range dest {
		if index >= len(r.values) {
			return errors.New("not enough values")
		}
		switch target := destination.(type) {
		case *bool:
			value, ok := r.values[index].(bool)
			if !ok {
				return errors.New("value is not a bool")
			}
			*target = value
		default:
			return errors.New("unsupported destination")
		}
	}
	return nil
}

type fakeRoleInspector struct {
	responses []fakeRoleRow
	calls     int
	queries   []string
}

func (i *fakeRoleInspector) QueryRow(_ context.Context, sql string, _ ...any) RowScanner {
	response := i.responses[min(i.calls, len(i.responses)-1)]
	i.calls++
	i.queries = append(i.queries, sql)
	return response
}

// readOnlySession is the answer of a session that authenticated as the plain
// runtime role: the session still runs as that role, reaches nothing
// privileged and is not shared with another login role.
func readOnlySession() []fakeRoleRow {
	return []fakeRoleRow{
		{values: []any{true, false, false}},
		{values: []any{false, false, false, false}},
	}
}

func TestRuntimeRoleVerifierAcceptsReadOnlyRole(t *testing.T) {
	inspector := &fakeRoleInspector{responses: readOnlySession()}

	if err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRoleVerifierRejectsSuperuser(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, true, false}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleIsSuperuser) {
		t.Fatalf("err = %v, want ErrRuntimeRoleIsSuperuser", err)
	}
}

func TestRuntimeRoleVerifierRejectsBypassRLS(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, false, true}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleBypassesRLS) {
		t.Fatalf("err = %v, want ErrRuntimeRoleBypassesRLS", err)
	}
}

func TestRuntimeRoleVerifierRejectsChangedSession(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{false, false, false, false}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleSessionChanged) {
		t.Fatalf("err = %v, want ErrRuntimeRoleSessionChanged", err)
	}
}

// A role that can assume the owner role or the administrative capability must
// be rejected even when it does not inherit the privileges yet.
func TestRuntimeRoleVerifierRejectsPrivilegedReachability(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, false, false, false}},
		{values: []any{true, false, false, false}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleReachesPrivilegedRole) {
		t.Fatalf("err = %v, want ErrRuntimeRoleReachesPrivilegedRole", err)
	}
}

// A role that can assume a role with BYPASSRLS must be rejected as well.
func TestRuntimeRoleVerifierRejectsReachableBypassRLS(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, false, false}},
		{values: []any{false, true, false, false}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleReachesPrivilegedRole) {
		t.Fatalf("err = %v, want ErrRuntimeRoleReachesPrivilegedRole", err)
	}
}

func TestRuntimeRoleVerifierRejectsWritableRole(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, false, false}},
		{values: []any{false, false, true, false}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleCanWrite) {
		t.Fatalf("err = %v, want ErrRuntimeRoleCanWrite", err)
	}
}

func TestRuntimeRoleVerifierRejectsSharedCredential(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, false, false}},
		{values: []any{false, false, false, true}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleSharedCredential) {
		t.Fatalf("err = %v, want ErrRuntimeRoleSharedCredential", err)
	}
}

func TestRuntimeRoleVerifierReportsUnknownRole(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{err: errors.New("connection lost")},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleUnknown) {
		t.Fatalf("err = %v, want ErrRuntimeRoleUnknown", err)
	}
}

func TestRuntimeRoleVerifierFailsClosedWithoutInspector(t *testing.T) {
	err := NewRuntimeRoleVerifier(nil).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleUnknown) {
		t.Fatalf("err = %v, want ErrRuntimeRoleUnknown", err)
	}
}

func TestRuntimeRoleVerifierCheckIncludesSharedCredential(t *testing.T) {
	inspector := &fakeRoleInspector{responses: readOnlySession()}

	if err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background()); err != nil {
		t.Fatal(err)
	}
	query := inspector.queries[1]
	if !strings.Contains(query, "assumers") {
		t.Fatalf("query %q does not mention the assumers CTE for detecting shared credentials", query)
	}
}

// The identity query has to read the authenticated role and compare it with
// the current one, so a session that changed the role cannot pass the check.
func TestRuntimeRoleVerifierChecksTheAuthenticatedSession(t *testing.T) {
	inspector := &fakeRoleInspector{responses: readOnlySession()}

	if err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background()); err != nil {
		t.Fatal(err)
	}
	query := inspector.queries[0]
	for _, fragment := range []string{
		"session_user = current_user",
		"rolsuper",
		"rolbypassrls",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query %q does not mention %q", query, fragment)
		}
	}
}
