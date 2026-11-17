package authn

import (
	"context"
	"errors"
	"net/http"

	"energyhub-universal/backend/internal/authz"
)

type principalContextKey struct{}

// Middleware authenticates a request with the injected verifier and exposes
// only the verified principal to downstream handlers.
func Middleware(verifier Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			principal, err := Authenticate(request.Context(), request, verifier)
			if err != nil {
				status := http.StatusUnauthorized
				if errors.Is(err, ErrVerifierUnavailable) {
					status = http.StatusInternalServerError
				}
				http.Error(writer, http.StatusText(status), status)
				return
			}

			next.ServeHTTP(writer, request.WithContext(withPrincipal(request.Context(), principal)))
		})
	}
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
