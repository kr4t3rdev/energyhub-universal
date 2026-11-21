package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"energyhub-universal/backend/internal/authn"
	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/site"
)

type routerOptions struct {
	verifier    authn.Verifier
	memberships authz.MembershipReader
	sites       *site.Service
	corsOrigin  string
}

// Option configures the router.
type Option func(*routerOptions)

// WithSites enables the site routes. They are only registered when the
// verifier, the membership reader and the site service are all present, so a
// half-configured deployment exposes no protected surface.
func WithSites(service *site.Service) Option {
	return func(options *routerOptions) {
		options.sites = service
	}
}

// WithAuthentication enables the protected routes. Routes are only registered
// when both the verifier and the membership reader are present, so a
// half-configured deployment exposes no protected surface.
func WithAuthentication(verifier authn.Verifier, memberships authz.MembershipReader) Option {
	return func(options *routerOptions) {
		options.verifier = verifier
		options.memberships = memberships
	}
}

// WithCORS allows browser requests from one exact origin. Empty origins leave
// CORS disabled; origin validation belongs to the deployment configuration.
func WithCORS(origin string) Option {
	return func(options *routerOptions) {
		options.corsOrigin = origin
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
	r.Use(corsMiddleware(settings.corsOrigin))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	if settings.verifier != nil && settings.memberships != nil {
		r.Route("/v1", func(r chi.Router) {
			// The protected responses depend on the verified subject, so no
			// cache may keep them. The headers are set before the
			// authentication runs, so the error envelopes carry them too.
			r.Use(privateResponseHeaders)
			r.Use(authn.Middleware(settings.verifier, authenticationErrorResponder))
			r.Get("/organizations", organizationsHandler(settings.memberships).ServeHTTP)
			if settings.sites != nil {
				authorizer := authz.NewMembershipAuthorizer(settings.memberships)
				r.Get("/me", meHandler(settings.memberships).ServeHTTP)
				r.Get("/organizations/{organizationId}/sites", listSitesHandler(authorizer, settings.sites).ServeHTTP)
				r.Get("/organizations/{organizationId}/sites/{siteId}/overview", siteOverviewHandler(authorizer, settings.sites).ServeHTTP)
				r.Get("/organizations/{organizationId}/sites/{siteId}/devices", listDevicesHandler(authorizer, settings.sites).ServeHTTP)
			}
		})
	}

	return r
}

// privateResponseHeaders marks the responses of the protected routes as
// uncacheable and tells the caches that they depend on the authorization
// header.
func privateResponseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		addVary(writer.Header(), "Authorization")
		next.ServeHTTP(writer, request)
	})
}

func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if allowedOrigin == "" {
				next.ServeHTTP(writer, request)
				return
			}

			addVary(writer.Header(), "Origin")
			origin := request.Header.Get("Origin")
			isAllowedOrigin := origin != "" && origin == allowedOrigin
			if isAllowedOrigin {
				writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			}

			requestedMethod := request.Header.Get("Access-Control-Request-Method")
			if request.Method != http.MethodOptions || requestedMethod == "" {
				next.ServeHTTP(writer, request)
				return
			}

			if !isAllowedOrigin || !corsMethodAllowed(requestedMethod) || !corsHeadersAllowed(request.Header.Get("Access-Control-Request-Headers")) {
				writer.WriteHeader(http.StatusForbidden)
				return
			}

			writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization")
			writer.WriteHeader(http.StatusNoContent)
		})
	}
}

func corsMethodAllowed(method string) bool {
	return method == http.MethodGet || method == http.MethodOptions
}

func corsHeadersAllowed(headers string) bool {
	for _, header := range strings.Split(headers, ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "", "accept", "authorization":
		default:
			return false
		}
	}
	return true
}

func addVary(headers http.Header, value string) {
	for _, field := range headers.Values("Vary") {
		for _, item := range strings.Split(field, ",") {
			item = strings.TrimSpace(item)
			if item == "*" || strings.EqualFold(item, value) {
				return
			}
		}
	}
	values := headers.Values("Vary")
	if len(values) == 0 {
		headers.Set("Vary", value)
		return
	}
	headers.Set("Vary", strings.Join(append(values, value), ", "))
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
