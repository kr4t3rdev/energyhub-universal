package main

import (
	"errors"
	"testing"
)

func TestMigrationDatabaseURLUsesTheMigrationVariable(t *testing.T) {
	url := migrationDatabaseURL(func(key string) string {
		switch key {
		case "MIGRATION_DATABASE_URL":
			return "postgres://migrator@127.0.0.1:5432/energyhub"
		case "DATABASE_URL":
			return "postgres://runtime@127.0.0.1:5432/energyhub"
		}
		return ""
	})
	if url != "postgres://migrator@127.0.0.1:5432/energyhub" {
		t.Fatalf("url = %q", url)
	}
}

func TestMigrationDatabaseURLIgnoresTheRuntimeVariable(t *testing.T) {
	url := migrationDatabaseURL(func(key string) string {
		if key == "DATABASE_URL" {
			return " postgres://runtime@127.0.0.1:5432/energyhub "
		}
		return ""
	})
	if url != "" {
		t.Fatalf("url = %q, want empty", url)
	}
}

func TestMigrationDatabaseURLIsEmptyWithoutConfiguration(t *testing.T) {
	if url := migrationDatabaseURL(func(string) string { return "" }); url != "" {
		t.Fatalf("url = %q, want empty", url)
	}
}

func TestMigrationDatabaseURLIsEmptyWithoutReader(t *testing.T) {
	if url := migrationDatabaseURL(nil); url != "" {
		t.Fatalf("url = %q, want empty", url)
	}
}

func TestValidateMigrationCredentialRejectsTheRuntimeRole(t *testing.T) {
	err := validateMigrationCredential("postgres://energyhub_runtime:secret@127.0.0.1:5432/energyhub")
	if !errors.Is(err, ErrRuntimeCredentialForMigrations) {
		t.Fatalf("err = %v, want ErrRuntimeCredentialForMigrations", err)
	}
}

func TestValidateMigrationCredentialAcceptsAnotherRole(t *testing.T) {
	if err := validateMigrationCredential("postgres://energyhub_migrator:secret@127.0.0.1:5432/energyhub"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMigrationCredentialAcceptsAConnectionWithoutUser(t *testing.T) {
	if err := validateMigrationCredential("postgres://127.0.0.1:5432/energyhub"); err != nil {
		t.Fatal(err)
	}
}
