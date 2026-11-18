package devoidc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The development issuer is a local tool, so it refuses anything that does
// not come from the machine and it never accepts an unbounded body. These
// checks guard the limits that keep it usable as a test fixture.

func TestTokenEndpointRejectsOversizedBody(t *testing.T) {
	issuer, _ := newTestIssuer(t)

	// The body is valid JSON so the decoder reads it whole and the size limit
	// is what rejects it.
	body := `{"client_id":"` + strings.Repeat("a", maxTokenRequestBytes) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	request.Host = "127.0.0.1:8089"
	request.RemoteAddr = "127.0.0.1:54321"
	response := httptest.NewRecorder()

	issuer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestTokenEndpointRejectsTrailingContent(t *testing.T) {
	issuer, _ := newTestIssuer(t)

	body := `{"client_id":"energyhub-dev","client_secret":"dev-secret","username":"admin","password":"admin-pass","audience":"https://api.energyhub.local"} {"extra":true}`
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	request.Host = "127.0.0.1:8089"
	request.RemoteAddr = "127.0.0.1:54321"
	response := httptest.NewRecorder()

	issuer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestTokenEndpointRejectsUnknownFields(t *testing.T) {
	issuer, _ := newTestIssuer(t)

	body := `{"client_id":"energyhub-dev","client_secret":"dev-secret","username":"admin","password":"admin-pass","audience":"https://api.energyhub.local","scope":"openid"}`
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	request.Host = "127.0.0.1:8089"
	request.RemoteAddr = "127.0.0.1:54321"
	response := httptest.NewRecorder()

	issuer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestIssuerRejectsRequestFromAnotherMachine(t *testing.T) {
	issuer, _ := newTestIssuer(t)

	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(`{}`))
	request.Host = "127.0.0.1:8089"
	request.RemoteAddr = "192.168.1.10:54321"
	response := httptest.NewRecorder()

	issuer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestTokenResponseIsNotCacheable(t *testing.T) {
	issuer, _ := newTestIssuer(t)

	body := `{"client_id":"energyhub-dev","client_secret":"dev-secret","username":"admin","password":"admin-pass","audience":"https://api.energyhub.local"}`
	request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(body))
	request.Host = "127.0.0.1:8089"
	request.RemoteAddr = "127.0.0.1:54321"
	response := httptest.NewRecorder()

	issuer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", cacheControl, "no-store")
	}
}

func TestIssuerRejectsLocalhostName(t *testing.T) {
	config := testConfig()
	config.Issuer = "http://localhost:8089"

	if _, err := New(config); err == nil {
		t.Fatal("err = nil, want a literal loopback address error")
	}
}
