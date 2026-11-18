package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"energyhub-universal/backend/internal/authz"
)

// The protected route depends on the verified subject, so no cache may keep
// its responses and every response must announce the authorization header it
// depends on. The check covers the success path and both error paths.

func TestProtectedRouteResponsesAreNotCacheable(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}

	tests := []struct {
		name          string
		verifier      *fakeVerifier
		memberships   *fakeMemberships
		authorization string
		wantStatus    int
	}{
		{
			name:          "success",
			verifier:      verifier,
			memberships:   &fakeMemberships{},
			authorization: "Bearer token-value",
			wantStatus:    http.StatusOK,
		},
		{
			name:        "missing token",
			verifier:    verifier,
			memberships: &fakeMemberships{},
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:          "invalid token",
			verifier:      &fakeVerifier{err: errors.New("token expired")},
			memberships:   &fakeMemberships{},
			authorization: "Bearer invalid-token",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "reader failure",
			verifier:      verifier,
			memberships:   &fakeMemberships{err: errors.New("database is down")},
			authorization: "Bearer token-value",
			wantStatus:    http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()

			NewRouter(WithAuthentication(test.verifier, test.memberships)).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
				t.Fatalf("Cache-Control = %q, want %q", cacheControl, "no-store")
			}
			if vary := response.Header().Get("Vary"); vary != "Authorization" {
				t.Fatalf("Vary = %q, want %q", vary, "Authorization")
			}
		})
	}
}
