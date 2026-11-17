package main

import (
	"errors"
	"strings"
	"testing"

	"energyhub-universal/backend/internal/authn"
)

func TestLoadAPIConfigAllowsHealthOnlyWithoutConfiguration(t *testing.T) {
	loaded, err := loadAPIConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if loaded.authenticationEnabled() {
		t.Fatal("authentication unexpectedly enabled")
	}
}

func TestLoadAPIConfigRejectsPartialAuthenticationConfiguration(t *testing.T) {
	tests := []map[string]string{
		{"AUTH0_ISSUER": "https://energyhub.eu.auth0.com/"},
		{"AUTH0_AUDIENCE": "https://api.energyhub.example"},
	}

	for _, values := range tests {
		_, err := loadAPIConfig(func(key string) string { return values[key] })
		if !errors.Is(err, ErrIncompleteAuthConfig) {
			t.Fatalf("err = %v, want ErrIncompleteAuthConfig", err)
		}
	}
}

func TestLoadAPIConfigRejectsInvalidIssuer(t *testing.T) {
	_, err := loadAPIConfig(func(key string) string {
		switch key {
		case "AUTH0_ISSUER":
			return "http://auth.example.test"
		case "AUTH0_AUDIENCE":
			return "https://api.energyhub.example"
		}
		return ""
	})
	if err == nil {
		t.Fatal("err = nil, want invalid configuration error")
	}
}

func TestLoadAPIConfigRequiresDatabaseURLWithAuthentication(t *testing.T) {
	_, err := loadAPIConfig(func(key string) string {
		switch key {
		case "AUTH0_ISSUER":
			return "https://energyhub.eu.auth0.com/"
		case "AUTH0_AUDIENCE":
			return "https://api.energyhub.example"
		}
		return ""
	})
	if !errors.Is(err, ErrIncompleteDatabaseConfig) {
		t.Fatalf("err = %v, want ErrIncompleteDatabaseConfig", err)
	}
}

func TestLoadAPIConfigRejectsDatabaseURLWithoutAuthentication(t *testing.T) {
	_, err := loadAPIConfig(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://runtime@127.0.0.1:5432/energyhub"
		}
		return ""
	})
	if !errors.Is(err, ErrIncompleteAuthConfig) {
		t.Fatalf("err = %v, want ErrIncompleteAuthConfig", err)
	}
}

func TestLoadAPIConfigAcceptsCompleteConfiguration(t *testing.T) {
	loaded, err := loadAPIConfig(func(key string) string {
		switch key {
		case "AUTH0_ISSUER":
			return "https://energyhub.eu.auth0.com/"
		case "AUTH0_AUDIENCE":
			return "https://api.energyhub.example"
		case "DATABASE_URL":
			return "postgres://runtime@127.0.0.1:5432/energyhub"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.authenticationEnabled() {
		t.Fatal("authentication unexpectedly disabled")
	}
	if loaded.databaseURL != "postgres://runtime@127.0.0.1:5432/energyhub" {
		t.Fatalf("database URL = %q", loaded.databaseURL)
	}
}

func TestLoadAPIConfigAcceptsLoopbackDevelopmentIssuer(t *testing.T) {
	loaded, err := loadAPIConfig(func(key string) string {
		switch key {
		case "AUTH0_ISSUER":
			return "http://127.0.0.1:8089"
		case "AUTH0_AUDIENCE":
			return "https://api.energyhub.local"
		case "DATABASE_URL":
			return "postgres://runtime@127.0.0.1:5432/energyhub"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.auth.Issuer != "http://127.0.0.1:8089" {
		t.Fatalf("issuer = %q", loaded.auth.Issuer)
	}
}

func TestLoadAPIConfigRejectsNilEnvironmentReader(t *testing.T) {
	if _, err := loadAPIConfig(nil); err == nil {
		t.Fatal("err = nil, want environment reader error")
	}
}

func TestAPIConfigTrimsValues(t *testing.T) {
	loaded, err := loadAPIConfig(func(key string) string {
		switch key {
		case "AUTH0_ISSUER":
			return "  https://energyhub.eu.auth0.com/  "
		case "AUTH0_AUDIENCE":
			return "  https://api.energyhub.example  "
		case "DATABASE_URL":
			return "  postgres://runtime@127.0.0.1:5432/energyhub  "
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.auth.Issuer != "https://energyhub.eu.auth0.com/" {
		t.Fatalf("issuer = %q", loaded.auth.Issuer)
	}
	if strings.TrimSpace(loaded.databaseURL) != loaded.databaseURL {
		t.Fatalf("database URL = %q, want trimmed", loaded.databaseURL)
	}
}

var _ = authn.Auth0Config{}
