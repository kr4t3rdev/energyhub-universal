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

func TestRuntimeRoleVerifierAcceptsReadOnlyRole(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{false, false}},
		{values: []any{false}},
	}}

	if err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRoleVerifierRejectsSuperuser(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{true, false}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleIsSuperuser) {
		t.Fatalf("err = %v, want ErrRuntimeRoleIsSuperuser", err)
	}
}

func TestRuntimeRoleVerifierRejectsBypassRLS(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{false, true}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleBypassesRLS) {
		t.Fatalf("err = %v, want ErrRuntimeRoleBypassesRLS", err)
	}
}

func TestRuntimeRoleVerifierRejectsWritableRole(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{false, false}},
		{values: []any{true}},
	}}

	err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background())
	if !errors.Is(err, ErrRuntimeRoleCanWrite) {
		t.Fatalf("err = %v, want ErrRuntimeRoleCanWrite", err)
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

func TestRuntimeRoleVerifierChecksEveryMutationPrivilege(t *testing.T) {
	inspector := &fakeRoleInspector{responses: []fakeRoleRow{
		{values: []any{false, false}},
		{values: []any{false}},
	}}

	if err := NewRuntimeRoleVerifier(inspector).VerifyRuntimeRole(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(inspector.queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(inspector.queries))
	}
	query := inspector.queries[1]
	for _, fragment := range []string{
		"public.organizations",
		"public.organization_memberships",
		"INSERT",
		"UPDATE",
		"DELETE",
		"TRUNCATE",
		"has_schema_privilege",
		"'app'",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query %q does not mention %q", query, fragment)
		}
	}
}
