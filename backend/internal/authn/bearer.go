package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"energyhub-universal/backend/internal/authz"
)

var (
	ErrMissingBearerToken   = errors.New("authorization bearer token is missing")
	ErrMalformedBearerToken = errors.New("authorization bearer token is malformed")
	ErrVerifierUnavailable  = errors.New("authentication verifier unavailable")
)

// Verifier is the provider-neutral boundary for a future OIDC/JWT verifier.
// Implementations must validate issuer, audience, signature and expiry before
// returning an authz.Principal.
type Verifier interface {
	Verify(ctx context.Context, token string) (authz.Principal, error)
}

// Authenticate composes request token extraction with an injected verifier.
// The verifier owns all cryptographic and issuer-specific decisions.
func Authenticate(ctx context.Context, request *http.Request, verifier Verifier) (authz.Principal, error) {
	token, err := BearerToken(request)
	if err != nil {
		return authz.Principal{}, err
	}
	if verifier == nil {
		return authz.Principal{}, ErrVerifierUnavailable
	}
	return verifier.Verify(ctx, token)
}

// BearerToken extracts exactly one Authorization bearer token.
// It does not validate or log the token.
func BearerToken(request *http.Request) (string, error) {
	if request == nil {
		return "", ErrMissingBearerToken
	}

	values := request.Header.Values("Authorization")
	if len(values) == 0 {
		return "", ErrMissingBearerToken
	}
	if len(values) != 1 {
		return "", ErrMalformedBearerToken
	}

	fields := strings.Fields(values[0])
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", ErrMalformedBearerToken
	}
	return fields[1], nil
}
