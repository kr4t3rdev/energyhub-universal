package main

import (
	"errors"
	"strings"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

func TestParseActionAcceptsCreateOrganization(t *testing.T) {
	action, err := parseAction([]string{"create-organization", "org-1", "Demo Org"})
	if err != nil {
		t.Fatal(err)
	}
	if action.kind != actionCreateOrganization {
		t.Fatalf("kind = %q, want %q", action.kind, actionCreateOrganization)
	}
	if action.organizationID != "org-1" || action.name != "Demo Org" {
		t.Fatalf("action = %#v", action)
	}
}

func TestParseActionAcceptsGrant(t *testing.T) {
	action, err := parseAction([]string{"grant", "org-1", "dev|user-1", "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if action.kind != actionGrant {
		t.Fatalf("kind = %q, want %q", action.kind, actionGrant)
	}
	if action.organizationID != "org-1" || action.subject != "dev|user-1" || action.role != contracts.RoleViewer {
		t.Fatalf("action = %#v", action)
	}
}

func TestParseActionAcceptsRevoke(t *testing.T) {
	action, err := parseAction([]string{"revoke", "org-1", "dev|user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if action.kind != actionRevoke {
		t.Fatalf("kind = %q, want %q", action.kind, actionRevoke)
	}
	if action.organizationID != "org-1" || action.subject != "dev|user-1" {
		t.Fatalf("action = %#v", action)
	}
}

func TestParseActionRejectsUnknownCommand(t *testing.T) {
	_, err := parseAction([]string{"drop-everything"})
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("err = %v, want ErrUnknownCommand", err)
	}
}

func TestParseActionRejectsMissingArguments(t *testing.T) {
	tests := [][]string{
		{},
		{"create-organization"},
		{"create-organization", "org-1"},
		{"grant"},
		{"grant", "org-1"},
		{"grant", "org-1", "dev|user-1"},
		{"revoke"},
		{"revoke", "org-1"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseAction(args); !errors.Is(err, ErrInvalidArguments) {
				t.Fatalf("err = %v, want ErrInvalidArguments", err)
			}
		})
	}
}

func TestParseActionRejectsUnknownRole(t *testing.T) {
	_, err := parseAction([]string{"grant", "org-1", "dev|user-1", "superuser"})
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("err = %v, want ErrInvalidRole", err)
	}
}

func TestLoadAdminConfigRequiresAdminDatabaseURL(t *testing.T) {
	_, err := loadAdminConfig(func(key string) string {
		if key == "SYSTEM_ADMIN_SUBJECT" {
			return "dev|admin"
		}
		return ""
	})
	if !errors.Is(err, ErrMissingAdminDatabaseURL) {
		t.Fatalf("err = %v, want ErrMissingAdminDatabaseURL", err)
	}
}

func TestLoadAdminConfigRequiresSystemAdminSubject(t *testing.T) {
	_, err := loadAdminConfig(func(key string) string {
		if key == "ADMIN_DATABASE_URL" {
			return "postgres://admin@127.0.0.1:5432/energyhub"
		}
		return ""
	})
	if !errors.Is(err, ErrMissingSystemAdminSubject) {
		t.Fatalf("err = %v, want ErrMissingSystemAdminSubject", err)
	}
}

func TestLoadAdminConfigReadsAndTrimsValues(t *testing.T) {
	loaded, err := loadAdminConfig(func(key string) string {
		switch key {
		case "ADMIN_DATABASE_URL":
			return "  postgres://admin@127.0.0.1:5432/energyhub  "
		case "SYSTEM_ADMIN_SUBJECT":
			return "  dev|admin  "
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.adminDatabaseURL != "postgres://admin@127.0.0.1:5432/energyhub" {
		t.Fatalf("admin database URL = %q", loaded.adminDatabaseURL)
	}
	if loaded.systemAdminSubject != "dev|admin" {
		t.Fatalf("system admin subject = %q", loaded.systemAdminSubject)
	}
}

func TestLoadAdminConfigRejectsNilEnvironmentReader(t *testing.T) {
	if _, err := loadAdminConfig(nil); err == nil {
		t.Fatal("err = nil, want environment reader error")
	}
}
