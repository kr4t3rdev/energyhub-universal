package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"energyhub-universal/backend/internal/authn"
	"energyhub-universal/backend/internal/authz"
)

type routerOptions struct {
	verifier    authn.Verifier
	memberships authz.MembershipReader
}

// Option configures the router.
type Option func(*routerOptions)

// WithAuthentication enables the protected routes. Routes are only registered
// when both the verifier and the membership reader are present, so a
// half-configured deployment exposes no protected surface.
func WithAuthentication(verifier authn.Verifier, memberships authz.MembershipReader) Option {
	return func(options *routerOptions) {
		options.verifier = verifier
		options.memberships = memberships
	}
}

func NewRouter(options ...Option) http.Handler {
	settings := routerOptions{}
	for _, option := range options {
		if option != nil {
			option(&settings)
		}
	}

	r := chi.NewRouter()
	r.Use(RequestIDMiddleware)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	if settings.verifier != nil && settings.memberships != nil {
		r.Route("/v1", func(r chi.Router) {
			r.Use(authn.Middleware(settings.verifier, authenticationErrorResponder))
			r.Get("/organizations", organizationsHandler(settings.memberships).ServeHTTP)
		})
	}

	return r
}

// authenticationErrorResponder keeps the error envelope consistent with the
// rest of the API and never exposes verifier details.
func authenticationErrorResponder(writer http.ResponseWriter, request *http.Request, status int, _ error) {
	code := "unauthenticated"
	message := "authentication is required"
	if status == http.StatusInternalServerError {
		code = "internal_error"
		message = "the request could not be completed"
	}
	requestID, _ := RequestIDFromContext(request.Context())
	WriteErrorResponse(writer, status, code, message, requestID)
}
