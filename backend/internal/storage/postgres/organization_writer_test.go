package postgres

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/authz"
)

func TestOrganizationWriterCreatesThroughTheAdminFunction(t *testing.T) {
	tx := &fakeMembershipTx{}
	db := &fakeDB{tx: tx}

	err := NewOrganizationWriter(db, "auth0|admin").Create(
		context.Background(),
		" auth0|admin ",
		" org-1 ",
		" Demo Org ",
	)
	if err != nil {
		t.Fatal(err)
	}
	if db.begins != 1 {
		t.Fatalf("begin calls = %d, want 1", db.begins)
	}
	if len(tx.execCalls) != 1 || tx.execCalls[0] != createOrganizationFunctionQuery {
		t.Fatalf("exec calls = %#v, want only the admin function call", tx.execCalls)
	}
	if len(tx.execArgs[0]) != 2 || tx.execArgs[0][0] != "org-1" || tx.execArgs[0][1] != "Demo Org" {
		t.Fatalf("create args = %#v", tx.execArgs[0])
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
}

func TestOrganizationWriterRejectsDifferentAdminBeforeTransaction(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewOrganizationWriter(db, "auth0|admin").Create(
		context.Background(),
		"auth0|other",
		"org-1",
		"Demo Org",
	)
	if !errors.Is(err, authz.ErrAdminForbidden) {
		t.Fatalf("err = %v, want ErrAdminForbidden", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestOrganizationWriterFailsClosedWithoutConfiguredAdmin(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewOrganizationWriter(db, " ").Create(
		context.Background(),
		"auth0|admin",
		"org-1",
		"Demo Org",
	)
	if !errors.Is(err, authz.ErrAdminNotConfigured) {
		t.Fatalf("err = %v, want ErrAdminNotConfigured", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestOrganizationWriterRejectsInvalidInputBeforeTransaction(t *testing.T) {
	tests := []struct {
		name    string
		org     string
		orgName string
	}{
		{name: "missing organization", org: " ", orgName: "Demo Org"},
		{name: "missing name", org: "org-1", orgName: " "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := &fakeDB{tx: &fakeMembershipTx{}}

			err := NewOrganizationWriter(db, "auth0|admin").Create(
				context.Background(),
				"auth0|admin",
				test.org,
				test.orgName,
			)
			if !errors.Is(err, authz.ErrInvalidOrganizationTarget) {
				t.Fatalf("err = %v, want ErrInvalidOrganizationTarget", err)
			}
			if db.begins != 0 {
				t.Fatalf("begin calls = %d, want 0", db.begins)
			}
		})
	}
}

func TestOrganizationWriterPropagatesBeginError(t *testing.T) {
	want := errors.New("pool exhausted")

	err := NewOrganizationWriter(&fakeDB{beginErr: want}, "auth0|admin").Create(
		context.Background(),
		"auth0|admin",
		"org-1",
		"Demo Org",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestOrganizationWriterRollsBackWhenFunctionFails(t *testing.T) {
	want := errors.New("caller is not a membership administrator")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 1}

	err := NewOrganizationWriter(&fakeDB{tx: tx}, "auth0|admin").Create(
		context.Background(),
		"auth0|admin",
		"org-1",
		"Demo Org",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if tx.commitCalled {
		t.Fatal("transaction was committed after a failed write")
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestOrganizationWriterPropagatesCommitError(t *testing.T) {
	want := errors.New("commit failed")
	tx := &fakeMembershipTx{commitErr: want}

	err := NewOrganizationWriter(&fakeDB{tx: tx}, "auth0|admin").Create(
		context.Background(),
		"auth0|admin",
		"org-1",
		"Demo Org",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back after a failed commit")
	}
}

func TestOrganizationWriterRejectsUnavailableDatabase(t *testing.T) {
	err := NewOrganizationWriter(nil, "auth0|admin").Create(
		context.Background(),
		"auth0|admin",
		"org-1",
		"Demo Org",
	)
	if !errors.Is(err, authz.ErrOrganizationWriterUnavailable) {
		t.Fatalf("err = %v, want ErrOrganizationWriterUnavailable", err)
	}
}

func TestOrganizationWriterRejectsNilReceiver(t *testing.T) {
	var writer *OrganizationWriter

	err := writer.Create(context.Background(), "auth0|admin", "org-1", "Demo Org")
	if !errors.Is(err, authz.ErrOrganizationWriterUnavailable) {
		t.Fatalf("err = %v, want ErrOrganizationWriterUnavailable", err)
	}
}
