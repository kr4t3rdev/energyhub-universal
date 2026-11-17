package postgres

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

func TestMembershipWriterGrantsInAnAdminTransaction(t *testing.T) {
	tx := &fakeMembershipTx{}
	db := &fakeDB{tx: tx}

	err := NewMembershipWriter(db, "auth0|admin").Grant(
		context.Background(),
		" auth0|admin ",
		" org-1 ",
		" auth0|user-1 ",
		contracts.RoleViewer,
	)
	if err != nil {
		t.Fatal(err)
	}
	if db.begins != 1 {
		t.Fatalf("begin calls = %d, want 1", db.begins)
	}
	if len(tx.execCalls) != 3 {
		t.Fatalf("exec calls = %#v, want subject, admin flag and upsert", tx.execCalls)
	}
	if tx.execCalls[0] != setMembershipSubjectQuery || len(tx.execArgs[0]) != 1 || tx.execArgs[0][0] != "auth0|admin" {
		t.Fatalf("subject setting = %q %#v", tx.execCalls[0], tx.execArgs[0])
	}
	if tx.execCalls[1] != setSystemAdminQuery {
		t.Fatalf("admin setting = %q", tx.execCalls[1])
	}
	if tx.execCalls[2] != upsertMembershipQuery {
		t.Fatalf("upsert = %q", tx.execCalls[2])
	}
	if len(tx.execArgs[2]) != 3 || tx.execArgs[2][0] != "org-1" || tx.execArgs[2][1] != "auth0|user-1" || tx.execArgs[2][2] != "viewer" {
		t.Fatalf("upsert args = %#v", tx.execArgs[2])
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
	if tx.rollbackCalled {
		t.Fatal("transaction was rolled back after success")
	}
}

func TestMembershipWriterRevokesInAnAdminTransaction(t *testing.T) {
	tx := &fakeMembershipTx{}

	err := NewMembershipWriter(&fakeDB{tx: tx}, "auth0|admin").Revoke(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.execCalls) != 3 || tx.execCalls[2] != deleteMembershipQuery {
		t.Fatalf("exec calls = %#v", tx.execCalls)
	}
	if len(tx.execArgs[2]) != 2 || tx.execArgs[2][0] != "org-1" || tx.execArgs[2][1] != "auth0|user-1" {
		t.Fatalf("delete args = %#v", tx.execArgs[2])
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
}

func TestMembershipWriterRejectsDifferentAdminBeforeTransaction(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewMembershipWriter(db, "auth0|admin").Grant(
		context.Background(),
		"auth0|other",
		"org-1",
		"auth0|user-1",
		contracts.RoleViewer,
	)
	if !errors.Is(err, authz.ErrAdminForbidden) {
		t.Fatalf("err = %v, want ErrAdminForbidden", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestMembershipWriterFailsClosedWithoutConfiguredAdmin(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewMembershipWriter(db, " ").Grant(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
		contracts.RoleViewer,
	)
	if !errors.Is(err, authz.ErrAdminNotConfigured) {
		t.Fatalf("err = %v, want ErrAdminNotConfigured", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestMembershipWriterRejectsInvalidInputBeforeTransaction(t *testing.T) {
	tests := []struct {
		name    string
		admin   string
		org     string
		subject string
		role    contracts.OrganizationRole
		want    error
	}{
		{name: "missing admin", admin: " ", org: "org-1", subject: "auth0|user-1", role: contracts.RoleViewer, want: authz.ErrUnauthenticated},
		{name: "missing organization", admin: "auth0|admin", org: " ", subject: "auth0|user-1", role: contracts.RoleViewer, want: authz.ErrInvalidMembershipTarget},
		{name: "missing target subject", admin: "auth0|admin", org: "org-1", subject: " ", role: contracts.RoleViewer, want: authz.ErrInvalidMembershipTarget},
		{name: "invalid role", admin: "auth0|admin", org: "org-1", subject: "auth0|user-1", role: contracts.OrganizationRole("superuser"), want: ErrInvalidMembershipRole},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := &fakeDB{tx: &fakeMembershipTx{}}
			err := NewMembershipWriter(db, "auth0|admin").Grant(
				context.Background(),
				test.admin,
				test.org,
				test.subject,
				test.role,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
			if db.begins != 0 {
				t.Fatalf("begin calls = %d, want 0", db.begins)
			}
		})
	}
}

func TestMembershipWriterPropagatesBeginError(t *testing.T) {
	want := errors.New("pool exhausted")

	err := NewMembershipWriter(&fakeDB{beginErr: want}, "auth0|admin").Revoke(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMembershipWriterRollsBackWhenSubjectSettingFails(t *testing.T) {
	want := errors.New("set_config failed")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 1}

	err := NewMembershipWriter(&fakeDB{tx: tx}, "auth0|admin").Grant(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
		contracts.RoleViewer,
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipWriterRollsBackWhenAdminFlagFails(t *testing.T) {
	want := errors.New("set_config failed")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 2}

	err := NewMembershipWriter(&fakeDB{tx: tx}, "auth0|admin").Grant(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
		contracts.RoleViewer,
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipWriterRollsBackWhenWriteFails(t *testing.T) {
	want := errors.New("foreign key violation")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 3}

	err := NewMembershipWriter(&fakeDB{tx: tx}, "auth0|admin").Grant(
		context.Background(),
		"auth0|admin",
		"missing-org",
		"auth0|user-1",
		contracts.RoleViewer,
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

func TestMembershipWriterPropagatesCommitError(t *testing.T) {
	want := errors.New("commit failed")
	tx := &fakeMembershipTx{commitErr: want}

	err := NewMembershipWriter(&fakeDB{tx: tx}, "auth0|admin").Revoke(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back after a failed commit")
	}
}

func TestMembershipWriterRevokeRejectsInvalidTarget(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewMembershipWriter(db, "auth0|admin").Revoke(context.Background(), "auth0|admin", "", "auth0|user-1")
	if !errors.Is(err, authz.ErrInvalidMembershipTarget) {
		t.Fatalf("err = %v, want ErrInvalidMembershipTarget", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestMembershipWriterRollsBackWhenDeleteFails(t *testing.T) {
	want := errors.New("delete failed")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 3}

	err := NewMembershipWriter(&fakeDB{tx: tx}, "auth0|admin").Revoke(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back")
	}
}

func TestMembershipWriterRejectsUnavailableDatabase(t *testing.T) {
	if err := NewMembershipWriter(nil, "auth0|admin").Grant(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
		contracts.RoleViewer,
	); !errors.Is(err, authz.ErrMembershipWriterUnavailable) {
		t.Fatalf("err = %v, want ErrMembershipWriterUnavailable", err)
	}
}

func TestMembershipWriterRejectsNilReceiver(t *testing.T) {
	var writer *MembershipWriter

	if err := writer.Revoke(
		context.Background(),
		"auth0|admin",
		"org-1",
		"auth0|user-1",
	); !errors.Is(err, authz.ErrMembershipWriterUnavailable) {
		t.Fatalf("err = %v, want ErrMembershipWriterUnavailable", err)
	}
}
