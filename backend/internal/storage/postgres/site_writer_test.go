package postgres

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/authz"
)

func TestSiteWriterCreatesSiteThroughTheAdminFunction(t *testing.T) {
	tx := &fakeMembershipTx{}
	db := &fakeDB{tx: tx}

	err := NewSiteWriter(db, "auth0|admin").CreateSite(
		context.Background(),
		" auth0|admin ",
		" org-1 ",
		" site-1 ",
		" HQ ",
		" Europe/Madrid ",
	)
	if err != nil {
		t.Fatal(err)
	}
	if db.begins != 1 {
		t.Fatalf("begin calls = %d, want 1", db.begins)
	}
	if len(tx.execCalls) != 1 || tx.execCalls[0] != createSiteFunctionQuery {
		t.Fatalf("exec calls = %#v, want only the admin function call", tx.execCalls)
	}
	if len(tx.execArgs[0]) != 4 || tx.execArgs[0][0] != "site-1" || tx.execArgs[0][1] != "org-1" || tx.execArgs[0][2] != "HQ" || tx.execArgs[0][3] != "Europe/Madrid" {
		t.Fatalf("create site args = %#v", tx.execArgs[0])
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
}

func TestSiteWriterCreatesDeviceThroughTheAdminFunction(t *testing.T) {
	tx := &fakeMembershipTx{}
	db := &fakeDB{tx: tx}

	err := NewSiteWriter(db, "auth0|admin").CreateDevice(
		context.Background(),
		" auth0|admin ",
		" org-1 ",
		" site-1 ",
		" device-1 ",
		" Main Meter ",
		" meter ",
	)
	if err != nil {
		t.Fatal(err)
	}
	if db.begins != 1 {
		t.Fatalf("begin calls = %d, want 1", db.begins)
	}
	if len(tx.execCalls) != 1 || tx.execCalls[0] != createSiteDeviceFunctionQuery {
		t.Fatalf("exec calls = %#v, want only the admin function call", tx.execCalls)
	}
	if len(tx.execArgs[0]) != 5 || tx.execArgs[0][0] != "device-1" || tx.execArgs[0][1] != "site-1" || tx.execArgs[0][2] != "org-1" || tx.execArgs[0][3] != "Main Meter" || tx.execArgs[0][4] != "meter" {
		t.Fatalf("create device args = %#v", tx.execArgs[0])
	}
	if !tx.commitCalled {
		t.Fatal("transaction was not committed")
	}
}

func TestSiteWriterRejectsDifferentAdminBeforeTransaction(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewSiteWriter(db, "auth0|admin").CreateSite(
		context.Background(),
		"auth0|other",
		"org-1",
		"site-1",
		"HQ",
		"Europe/Madrid",
	)
	if !errors.Is(err, authz.ErrAdminForbidden) {
		t.Fatalf("err = %v, want ErrAdminForbidden", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestSiteWriterFailsClosedWithoutConfiguredAdmin(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewSiteWriter(db, " ").CreateDevice(
		context.Background(),
		"auth0|admin",
		"org-1",
		"site-1",
		"device-1",
		"Meter",
		"meter",
	)
	if !errors.Is(err, authz.ErrAdminNotConfigured) {
		t.Fatalf("err = %v, want ErrAdminNotConfigured", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestSiteWriterRejectsInvalidInputBeforeTransaction(t *testing.T) {
	siteCases := []struct {
		name     string
		org      string
		siteID   string
		siteName string
		timezone string
	}{
		{name: "missing organization", org: " ", siteID: "site-1", siteName: "HQ", timezone: "Europe/Madrid"},
		{name: "missing site id", org: "org-1", siteID: " ", siteName: "HQ", timezone: "Europe/Madrid"},
		{name: "missing name", org: "org-1", siteID: "site-1", siteName: " ", timezone: "Europe/Madrid"},
		{name: "missing timezone", org: "org-1", siteID: "site-1", siteName: "HQ", timezone: " "},
	}
	for _, test := range siteCases {
		t.Run("site "+test.name, func(t *testing.T) {
			db := &fakeDB{tx: &fakeMembershipTx{}}
			err := NewSiteWriter(db, "auth0|admin").CreateSite(
				context.Background(),
				"auth0|admin",
				test.org,
				test.siteID,
				test.siteName,
				test.timezone,
			)
			if !errors.Is(err, authz.ErrInvalidSiteTarget) {
				t.Fatalf("err = %v, want ErrInvalidSiteTarget", err)
			}
			if db.begins != 0 {
				t.Fatalf("begin calls = %d, want 0", db.begins)
			}
		})
	}
	deviceCases := []struct {
		name       string
		org        string
		siteID     string
		deviceID   string
		deviceName string
		kind       string
	}{
		{name: "missing organization", org: " ", siteID: "site-1", deviceID: "device-1", deviceName: "Meter", kind: "meter"},
		{name: "missing site id", org: "org-1", siteID: " ", deviceID: "device-1", deviceName: "Meter", kind: "meter"},
		{name: "missing device id", org: "org-1", siteID: "site-1", deviceID: " ", deviceName: "Meter", kind: "meter"},
		{name: "missing name", org: "org-1", siteID: "site-1", deviceID: "device-1", deviceName: " ", kind: "meter"},
		{name: "missing kind", org: "org-1", siteID: "site-1", deviceID: "device-1", deviceName: "Meter", kind: " "},
	}
	for _, test := range deviceCases {
		t.Run("device "+test.name, func(t *testing.T) {
			db := &fakeDB{tx: &fakeMembershipTx{}}
			err := NewSiteWriter(db, "auth0|admin").CreateDevice(
				context.Background(),
				"auth0|admin",
				test.org,
				test.siteID,
				test.deviceID,
				test.deviceName,
				test.kind,
			)
			if !errors.Is(err, authz.ErrInvalidSiteTarget) {
				t.Fatalf("err = %v, want ErrInvalidSiteTarget", err)
			}
			if db.begins != 0 {
				t.Fatalf("begin calls = %d, want 0", db.begins)
			}
		})
	}
}

func TestSiteWriterRejectsMissingAdminSubject(t *testing.T) {
	db := &fakeDB{tx: &fakeMembershipTx{}}

	err := NewSiteWriter(db, "auth0|admin").CreateSite(
		context.Background(),
		" ",
		"org-1",
		"site-1",
		"HQ",
		"Europe/Madrid",
	)
	if !errors.Is(err, authz.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if db.begins != 0 {
		t.Fatalf("begin calls = %d, want 0", db.begins)
	}
}

func TestSiteWriterPropagatesBeginError(t *testing.T) {
	want := errors.New("pool exhausted")

	err := NewSiteWriter(&fakeDB{beginErr: want}, "auth0|admin").CreateDevice(
		context.Background(),
		"auth0|admin",
		"org-1",
		"site-1",
		"device-1",
		"Meter",
		"meter",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestSiteWriterRollsBackWhenFunctionFails(t *testing.T) {
	want := errors.New("caller is not a membership administrator")
	tx := &fakeMembershipTx{execErr: want, failOnExec: 1}

	err := NewSiteWriter(&fakeDB{tx: tx}, "auth0|admin").CreateSite(
		context.Background(),
		"auth0|admin",
		"org-1",
		"site-1",
		"HQ",
		"Europe/Madrid",
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

func TestSiteWriterPropagatesCommitError(t *testing.T) {
	want := errors.New("commit failed")
	tx := &fakeMembershipTx{commitErr: want}

	err := NewSiteWriter(&fakeDB{tx: tx}, "auth0|admin").CreateDevice(
		context.Background(),
		"auth0|admin",
		"org-1",
		"site-1",
		"device-1",
		"Meter",
		"meter",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !tx.rollbackCalled {
		t.Fatal("transaction was not rolled back after a failed commit")
	}
}

func TestSiteWriterRejectsUnavailableDatabase(t *testing.T) {
	err := NewSiteWriter(nil, "auth0|admin").CreateSite(
		context.Background(),
		"auth0|admin",
		"org-1",
		"site-1",
		"HQ",
		"Europe/Madrid",
	)
	if !errors.Is(err, authz.ErrSiteWriterUnavailable) {
		t.Fatalf("err = %v, want ErrSiteWriterUnavailable", err)
	}
}

func TestSiteWriterRejectsNilReceiver(t *testing.T) {
	var writer *SiteWriter

	err := writer.CreateDevice(context.Background(), "auth0|admin", "org-1", "site-1", "device-1", "Meter", "meter")
	if !errors.Is(err, authz.ErrSiteWriterUnavailable) {
		t.Fatalf("err = %v, want ErrSiteWriterUnavailable", err)
	}
}
