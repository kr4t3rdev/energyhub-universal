package postgres

import (
	"errors"
	"strings"
	"testing"
)

// The connection string reaches the error messages of the drivers, and those
// messages end up in the startup log. The password of the administrative
// credential must never be part of them.

func TestRedactConnectionErrorRemovesThePassword(t *testing.T) {
	const passphrase = "super-secret-admin-password"
	databaseURL := "postgres://energyhub_admin:" + passphrase + "@127.0.0.1:5432/energyhub?sslmode=disable"

	err := RedactConnectionError(errors.New("failed to connect: "+databaseURL), databaseURL)

	if err == nil {
		t.Fatal("err = nil, want a redacted error")
	}
	if strings.Contains(err.Error(), passphrase) {
		t.Fatalf("error still holds the password: %q", err.Error())
	}
	if !strings.Contains(err.Error(), RedactedPassword) {
		t.Fatalf("error = %q, want the redaction marker", err.Error())
	}
}

func TestRedactConnectionErrorKeepsTheOriginalMessage(t *testing.T) {
	const passphrase = "super-secret-admin-password"
	databaseURL := "postgres://energyhub_admin:" + passphrase + "@127.0.0.1:5432/energyhub?sslmode=disable"

	err := RedactConnectionError(errors.New("password authentication failed"), databaseURL)

	if err == nil || err.Error() != "password authentication failed" {
		t.Fatalf("err = %v, want the original message", err)
	}
}

func TestRedactConnectionErrorWithoutAPassword(t *testing.T) {
	databaseURL := "postgres://energyhub_admin@127.0.0.1:5432/energyhub?sslmode=disable"
	original := errors.New("connection refused")

	err := RedactConnectionError(original, databaseURL)

	if err == nil {
		t.Fatal("err = nil, want a redacted error")
	}
	if strings.Contains(err.Error(), original.Error()) {
		t.Fatalf("error still holds the raw message: %q", err.Error())
	}
	if err.Error() != "database connection failed: unable to redact credentials" {
		t.Fatalf("err = %v, want a generic redacted message", err)
	}
}

func TestRedactConnectionErrorIgnoresANilError(t *testing.T) {
	if err := RedactConnectionError(nil, "postgres://user:passphrase@host/db"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestRedactConnectionErrorHandlesAMalformedURL(t *testing.T) {
	original := errors.New("connection refused")

	err := RedactConnectionError(original, "://not a url")

	if err == nil {
		t.Fatal("err = nil, want a redacted error")
	}
	if strings.Contains(err.Error(), original.Error()) {
		t.Fatalf("error still holds the raw message: %q", err.Error())
	}
	if err.Error() != "database connection failed: unable to redact credentials" {
		t.Fatalf("err = %v, want a generic redacted message", err)
	}
}
