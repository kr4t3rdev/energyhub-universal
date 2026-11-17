package authn

import (
	"context"
	"errors"
	"net/http"

	"energyhub-universal/backend/internal/authz"
)

type principalContextKey struct{}

// ErrorResponder writes the response for a failed authentication. The error is
// passed for logging by the caller, never for the client.
type ErrorResponder func(writer http.ResponseWriter, request *http.Request, status int, err error)

// Middleware authenticates a request with the injected verifier and exposes
// only the verified principal to downstream handlers. An optional responder
// replaces the default plain-text error response.
func Middleware(verifier Verifier, responders ...ErrorResponder) func(http.Handler) http.Handler {
	responder := defaultErrorResponder
	if len(responders) > 0 && responders[0] != nil {
		responder = responders[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			principal, err := Authenticate(request.Context(), request, verifier)
			if err != nil {
				status := http.StatusUnauthorized
				if errors.Is(err, ErrVerifierUnavailable) {
					status = http.StatusInternalServerError
				}
				responder(writer, request, status, err)
				return
			}

			next.ServeHTTP(writer, request.WithContext(withPrincipal(request.Context(), principal)))
		})
	}
}

func defaultErrorResponder(writer http.ResponseWriter, _ *http.Request, status int, _ error) {
	http.Error(writer, http.StatusText(status), status)
}

func withPrincipal(ctx context.Context, principal authz.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (authz.Principal, bool) {
	if ctx == nil {
		return authz.Principal{}, false
	}
	principal, ok := ctx.Value(principalContextKey{}).(authz.Principal)
	return principal, ok
}
