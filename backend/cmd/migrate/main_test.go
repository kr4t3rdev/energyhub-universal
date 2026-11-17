package main

import (
	"testing"
)

func TestMigrationDatabaseURLPrefersMigrationVariable(t *testing.T) {
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

func TestMigrationDatabaseURLFallsBackToDatabaseURL(t *testing.T) {
	url := migrationDatabaseURL(func(key string) string {
		if key == "DATABASE_URL" {
			return " postgres://runtime@127.0.0.1:5432/energyhub "
		}
		return ""
	})
	if url != "postgres://runtime@127.0.0.1:5432/energyhub" {
		t.Fatalf("url = %q", url)
	}
}

func TestMigrationDatabaseURLIsEmptyWithoutConfiguration(t *testing.T) {
	if url := migrationDatabaseURL(func(string) string { return "" }); url != "" {
		t.Fatalf("url = %q, want empty", url)
	}
}
