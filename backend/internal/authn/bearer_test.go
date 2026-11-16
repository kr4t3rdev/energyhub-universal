package authn

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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
