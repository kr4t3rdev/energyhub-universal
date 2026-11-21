package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"energyhub-universal/backend/internal/authz"
)

const testAllowedOrigin = "https://energyhub.kr4t3rdev.xyz"

func TestCORSAllowedOriginCanReadProtectedUnauthorizedResponse(t *testing.T) {
	handler := NewRouter(
		WithCORS(testAllowedOrigin),
		WithAuthentication(&fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}, &fakeMemberships{}),
	)
	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Origin", testAllowedOrigin)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != testAllowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, testAllowedOrigin)
	}
	assertVaryContains(t, response.Header(), "Origin")
	assertVaryContains(t, response.Header(), "Authorization")
}

func TestCORSPreflightBypassesAuthentication(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	handler := NewRouter(WithCORS(testAllowedOrigin), WithAuthentication(verifier, &fakeMemberships{}))
	request := httptest.NewRequest(http.MethodOptions, "/v1/organizations", nil)
	request.Header.Set("Origin", testAllowedOrigin)
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != testAllowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, testAllowedOrigin)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", got, "GET, OPTIONS")
	}
	if got := strings.ToLower(response.Header().Get("Access-Control-Allow-Headers")); got != "accept, authorization" {
		t.Fatalf("Access-Control-Allow-Headers = %q, want %q", got, "accept, authorization")
	}
	if verifier.calls != 0 {
		t.Fatalf("verifier calls = %d, want 0", verifier.calls)
	}
	assertVaryContains(t, response.Header(), "Origin")
}

func TestCORSDoesNotAllowUnconfiguredOrigin(t *testing.T) {
	handler := NewRouter(
		WithCORS(testAllowedOrigin),
		WithAuthentication(&fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}, &fakeMemberships{}),
	)
	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want no CORS grant", got)
	}
}

func assertVaryContains(t *testing.T, headers http.Header, value string) {
	t.Helper()
	for _, field := range headers.Values("Vary") {
		for _, item := range strings.Split(field, ",") {
			if strings.EqualFold(strings.TrimSpace(item), value) {
				return
			}
		}
	}
	t.Fatalf("Vary = %#v, want %q", headers.Values("Vary"), value)
}
