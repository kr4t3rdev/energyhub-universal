package authn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"energyhub-universal/backend/internal/authz"
)

func TestMiddlewareAddsVerifiedPrincipalToContext(t *testing.T) {
	verifier := &fakeVerifier{
		principal: authz.Principal{Subject: "user-1"},
	}
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := PrincipalFromContext(request.Context())
		if !ok {
			t.Fatal("principal missing from context")
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(principal.Subject))
	})

	response := serveMiddleware(verifier, next, "Bearer token-value")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if response.Body.String() != "user-1" {
		t.Fatalf("body = %q, want user-1", response.Body.String())
	}
}

func TestMiddlewareRejectsUnauthenticatedRequestWithoutCallingNext(t *testing.T) {
	verifier := &fakeVerifier{}
	nextCalls := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalls++
	})

	response := serveMiddleware(verifier, next, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if nextCalls != 0 || verifier.calls != 0 {
		t.Fatalf("next calls = %d, verifier calls = %d, want 0/0", nextCalls, verifier.calls)
	}
}

func TestMiddlewareDoesNotExposeVerifierErrors(t *testing.T) {
	verifier := &fakeVerifier{err: errors.New("private verifier detail")}
	response := serveMiddleware(verifier, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "Bearer token-value")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if response.Body.String() == "private verifier detail\n" {
		t.Fatal("verifier error leaked to response")
	}
}

func TestMiddlewareReportsUnavailableVerifierAsServerError(t *testing.T) {
	response := serveMiddleware(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "Bearer token-value")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
}

func TestPrincipalFromContextReportsMissingValue(t *testing.T) {
	if _, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatal("principal unexpectedly present")
	}
}

func TestPrincipalFromContextRejectsNilContext(t *testing.T) {
	if _, ok := PrincipalFromContext(nil); ok {
		t.Fatal("principal unexpectedly present")
	}
}

func serveMiddleware(verifier Verifier, next http.Handler, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	Middleware(verifier)(next).ServeHTTP(response, request)
	return response
}
