package main

import (
	"strings"
	"testing"
)

func TestLoadConfigUsesDevelopmentDefaults(t *testing.T) {
	loaded, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if loaded.addr != defaultAddr {
		t.Fatalf("addr = %q, want %q", loaded.addr, defaultAddr)
	}
	if loaded.audience != defaultAudience {
		t.Fatalf("audience = %q, want %q", loaded.audience, defaultAudience)
	}
	if loaded.adminSubject != defaultAdminSubject {
		t.Fatalf("admin subject = %q, want %q", loaded.adminSubject, defaultAdminSubject)
	}
	if loaded.userSubject != defaultUserSubject {
		t.Fatalf("user subject = %q, want %q", loaded.userSubject, defaultUserSubject)
	}
}

func TestLoadConfigReadsEnvironmentOverrides(t *testing.T) {
	values := map[string]string{
		"DEV_OIDC_ADDR":          "127.0.0.1:9999",
		"DEV_OIDC_AUDIENCE":      "https://api.other.local",
		"DEV_OIDC_CLIENT_ID":     "other-client",
		"DEV_OIDC_CLIENT_SECRET": "other-secret",
		"DEV_OIDC_ADMIN_SUBJECT": "dev|root",
	}

	loaded, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if loaded.addr != "127.0.0.1:9999" {
		t.Fatalf("addr = %q", loaded.addr)
	}
	if loaded.audience != "https://api.other.local" {
		t.Fatalf("audience = %q", loaded.audience)
	}
	if loaded.clientID != "other-client" || loaded.clientSecret != "other-secret" {
		t.Fatalf("client = %q/%q", loaded.clientID, loaded.clientSecret)
	}
	if loaded.adminSubject != "dev|root" {
		t.Fatalf("admin subject = %q", loaded.adminSubject)
	}
}

func TestLoadConfigRejectsNonLoopbackAddress(t *testing.T) {
	_, err := loadConfig(func(key string) string {
		if key == "DEV_OIDC_ADDR" {
			return "0.0.0.0:8089"
		}
		return ""
	})
	if err == nil {
		t.Fatal("err = nil, want non-loopback address error")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("err = %v, want loopback guidance", err)
	}
}

func TestLoadConfigRejectsMalformedAddress(t *testing.T) {
	_, err := loadConfig(func(key string) string {
		if key == "DEV_OIDC_ADDR" {
			return "8089"
		}
		return ""
	})
	if err == nil {
		t.Fatal("err = nil, want malformed address error")
	}
}

func TestLoadConfigRejectsNilEnvironmentReader(t *testing.T) {
	if _, err := loadConfig(nil); err == nil {
		t.Fatal("err = nil, want environment reader error")
	}
}

func TestValidateLoopbackAddrRejectsLocalhostName(t *testing.T) {
	if err := validateLoopbackAddr("localhost:8089"); err == nil {
		t.Fatal("err = nil, want a literal loopback address error")
	}
}

func TestValidateLoopbackAddrAcceptsLoopbackIP(t *testing.T) {
	if err := validateLoopbackAddr("127.0.0.1:8089"); err != nil {
		t.Fatal(err)
	}
}
