package authn

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"energyhub-universal/backend/internal/authz"
)

func TestBearerTokenAcceptsCaseInsensitiveScheme(t *testing.T) {
	request := newRequestWithAuthorization("bEaReR token-value")

	token, err := BearerToken(request)
	if err != nil {
		t.Fatal(err)
	}
	if token != "token-value" {
		t.Fatalf("token = %q, want token-value", token)
	}
}

func TestBearerTokenRejectsMissingOrMalformedHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   error
	}{
		{name: "missing", want: ErrMissingBearerToken},
		{name: "wrong scheme", header: "Basic abc", want: ErrMalformedBearerToken},
		{name: "empty token", header: "Bearer", want: ErrMalformedBearerToken},
		{name: "token with whitespace", header: "Bearer token value", want: ErrMalformedBearerToken},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newRequestWithAuthorization(test.header)

			_, err := BearerToken(request)
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestBearerTokenRejectsMultipleAuthorizationHeaders(t *testing.T) {
	request := httptestRequest()
	request.Header.Add("Authorization", "Bearer first")
	request.Header.Add("Authorization", "Bearer second")

	_, err := BearerToken(request)
	if !errors.Is(err, ErrMalformedBearerToken) {
		t.Fatalf("err = %v, want ErrMalformedBearerToken", err)
	}
}

func TestBearerTokenRejectsNilRequest(t *testing.T) {
	_, err := BearerToken(nil)
	if !errors.Is(err, ErrMissingBearerToken) {
		t.Fatalf("err = %v, want ErrMissingBearerToken", err)
	}
}

type fakeVerifier struct {
	token     string
	principal authz.Principal
	err       error
	calls     int
}

func (f *fakeVerifier) Verify(_ context.Context, token string) (authz.Principal, error) {
	f.calls++
	f.token = token
	return f.principal, f.err
}

func TestAuthenticatePassesExtractedTokenToVerifier(t *testing.T) {
	verifier := &fakeVerifier{
		principal: authz.Principal{Subject: "user-1"},
	}

	principal, err := Authenticate(
		context.Background(),
		newRequestWithAuthorization("Bearer token-value"),
		verifier,
	)
	if err != nil {
		t.Fatal(err)
	}
	if verifier.calls != 1 || verifier.token != "token-value" {
		t.Fatalf("verifier = %#v, want one call with token-value", verifier)
	}
	if principal.Subject != "user-1" {
		t.Fatalf("subject = %q, want user-1", principal.Subject)
	}
}

func TestAuthenticateStopsBeforeVerifierForMalformedRequest(t *testing.T) {
	verifier := &fakeVerifier{}

	_, err := Authenticate(context.Background(), newRequestWithAuthorization("Basic abc"), verifier)
	if !errors.Is(err, ErrMalformedBearerToken) {
		t.Fatalf("err = %v, want ErrMalformedBearerToken", err)
	}
	if verifier.calls != 0 {
		t.Fatalf("verifier calls = %d, want 0", verifier.calls)
	}
}

func TestAuthenticatePropagatesVerifierError(t *testing.T) {
	want := errors.New("token rejected")
	verifier := &fakeVerifier{err: want}

	_, err := Authenticate(
		context.Background(),
		newRequestWithAuthorization("Bearer token-value"),
		verifier,
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestAuthenticateRejectsUnavailableVerifier(t *testing.T) {
	_, err := Authenticate(
		context.Background(),
		newRequestWithAuthorization("Bearer token-value"),
		nil,
	)
	if !errors.Is(err, ErrVerifierUnavailable) {
		t.Fatalf("err = %v, want ErrVerifierUnavailable", err)
	}
}

func newRequestWithAuthorization(value string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if value != "" {
		request.Header.Set("Authorization", value)
	}
	return request
}

func httptestRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}
