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
)

// Verifier is the provider-neutral boundary for a future OIDC/JWT verifier.
// Implementations must validate issuer, audience, signature and expiry before
// returning an authz.Principal.
type Verifier interface {
	Verify(ctx context.Context, token string) (authz.Principal, error)
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
