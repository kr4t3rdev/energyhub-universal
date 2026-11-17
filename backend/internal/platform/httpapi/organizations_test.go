package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

type fakeVerifier struct {
	principal authz.Principal
	err       error
	calls     int
}

func (f *fakeVerifier) Verify(_ context.Context, _ string) (authz.Principal, error) {
	f.calls++
	if f.err != nil {
		return authz.Principal{}, f.err
	}
	return f.principal, nil
}

type fakeMemberships struct {
	organizations []contracts.OrganizationSummary
	err           error
	subjects      []string
}

func (f *fakeMemberships) ListOrganizations(_ context.Context, subject string) ([]contracts.OrganizationSummary, error) {
	f.subjects = append(f.subjects, subject)
	if f.err != nil {
		return nil, f.err
	}
	return f.organizations, nil
}

func TestRouterWithoutAuthenticationHasNoProtectedRoutes(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/organizations", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestOrganizationsRouteRequiresAuthentication(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	memberships := &fakeMemberships{}

	response := httptest.NewRecorder()
	NewRouter(WithAuthentication(verifier, memberships)).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/v1/organizations", nil),
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if verifier.calls != 0 {
		t.Fatalf("verifier calls = %d, want 0", verifier.calls)
	}
	if len(memberships.subjects) != 0 {
		t.Fatalf("reader calls = %#v, want none", memberships.subjects)
	}
}

func TestOrganizationsRouteRejectsInvalidToken(t *testing.T) {
	verifier := &fakeVerifier{err: errors.New("token expired")}
	memberships := &fakeMemberships{}

	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response := httptest.NewRecorder()
	NewRouter(WithAuthentication(verifier, memberships)).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if len(memberships.subjects) != 0 {
		t.Fatalf("reader calls = %#v, want none", memberships.subjects)
	}
	var envelope contracts.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if envelope.Code == "" || envelope.RequestID == "" {
		t.Fatalf("error envelope = %#v, want code and request id", envelope)
	}
}

func TestOrganizationsRouteReturnsCallerOrganizations(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	memberships := &fakeMemberships{
		organizations: []contracts.OrganizationSummary{
			{ID: "org-1", Name: "Demo Org", Role: contracts.RoleViewer},
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Authorization", "Bearer token-value")
	response := httptest.NewRecorder()
	NewRouter(WithAuthentication(verifier, memberships)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if len(memberships.subjects) != 1 || memberships.subjects[0] != "dev|user-1" {
		t.Fatalf("reader subjects = %#v, want [dev|user-1]", memberships.subjects)
	}
	var body contracts.CurrentUser
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if body.Subject != "dev|user-1" {
		t.Fatalf("subject = %q, want dev|user-1", body.Subject)
	}
	if len(body.Organizations) != 1 || body.Organizations[0].ID != "org-1" {
		t.Fatalf("organizations = %#v", body.Organizations)
	}
}

func TestOrganizationsRouteHidesReaderFailures(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	memberships := &fakeMemberships{err: errors.New("connection refused to 10.0.0.5")}

	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Authorization", "Bearer token-value")
	response := httptest.NewRecorder()
	NewRouter(WithAuthentication(verifier, memberships)).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if response.Body.String() == "" {
		t.Fatal("body is empty")
	}
	if contains(response.Body.String(), "10.0.0.5") {
		t.Fatalf("body leaked internal detail: %s", response.Body.String())
	}
	var envelope contracts.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if envelope.RequestID == "" {
		t.Fatal("error envelope has no request id")
	}
}

func TestOrganizationsRouteIsNotRegisteredWithoutMembershipReader(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}

	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Authorization", "Bearer token-value")
	response := httptest.NewRecorder()
	NewRouter(WithAuthentication(verifier, nil)).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestOrganizationsRouteIsNotRegisteredWithoutVerifier(t *testing.T) {
	memberships := &fakeMemberships{}

	request := httptest.NewRequest(http.MethodGet, "/v1/organizations", nil)
	request.Header.Set("Authorization", "Bearer token-value")
	response := httptest.NewRecorder()
	NewRouter(WithAuthentication(nil, memberships)).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if len(memberships.subjects) != 0 {
		t.Fatalf("reader calls = %#v, want none", memberships.subjects)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
