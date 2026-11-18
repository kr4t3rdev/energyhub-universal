package main

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/storage/postgres"
)

// fakeRoleInspector answers the query that reads the effective role.
type fakeRoleInspector struct {
	role string
	err  error
}

func (i fakeRoleInspector) QueryRow(_ context.Context, _ string, _ ...any) postgres.RowScanner {
	return fakeRoleRow{role: i.role, err: i.err}
}

type fakeRoleRow struct {
	role string
	err  error
}

func (r fakeRoleRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for _, destination := range dest {
		target, ok := destination.(*string)
		if !ok {
			return errors.New("unsupported destination")
		}
		*target = r.role
	}
	return nil
}

// The URL userinfo is only a hint: the connection may resolve the role from
// the environment or from the server configuration. The role that is really
// connected is the one that would own the objects, so it is the one that has
// to be rejected.
func TestConnectedRoleReadsTheEffectiveRole(t *testing.T) {
	role, err := connectedRole(context.Background(), fakeRoleInspector{role: "energyhub_migrator"})
	if err != nil {
		t.Fatal(err)
	}
	if role != "energyhub_migrator" {
		t.Fatalf("role = %q, want %q", role, "energyhub_migrator")
	}
}

func TestConnectedRoleFailsClosedWhenTheRoleIsUnknown(t *testing.T) {
	if _, err := connectedRole(context.Background(), fakeRoleInspector{err: errors.New("connection lost")}); err == nil {
		t.Fatal("err = nil, want an inspection error")
	}
}

func TestConnectedRoleRequiresAConnection(t *testing.T) {
	if _, err := connectedRole(context.Background(), nil); err == nil {
		t.Fatal("err = nil, want a missing connection error")
	}
}
