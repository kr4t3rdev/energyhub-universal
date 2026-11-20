package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"energyhub-universal/backend/internal/authn"
	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/site"
)

// siteIDPattern accepts only the canonical UUID text form. Anything else is
// not a site the caller can see, so it maps to 404 without reaching the
// service or the database.
var siteIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// meHandler returns the same CurrentUser payload as /v1/organizations: the
// subject and its organizations. It exists so clients can confirm who they
// are without listing anything else.
func meHandler(memberships authz.MembershipReader) http.Handler {
	return organizationsHandler(memberships)
}

// resolveScope converts the verified principal into the tenant scope of the
// organization in the path. Membership is checked before any site is looked
// up, so a non-member learns nothing about the sites of that organization.
func resolveScope(writer http.ResponseWriter, request *http.Request, authorizer *authz.MembershipAuthorizer) (site.Scope, bool) {
	principal, ok := authn.PrincipalFromContext(request.Context())
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return site.Scope{}, false
	}
	if authorizer == nil {
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
		return site.Scope{}, false
	}
	scope, err := authorizer.ScopeForOrganization(request.Context(), principal, chi.URLParam(request, "organizationId"))
	if err != nil {
		if errors.Is(err, authz.ErrOrganizationForbidden) || errors.Is(err, authz.ErrInvalidOrganization) {
			writeError(writer, request, http.StatusForbidden, "forbidden", "the organization is not accessible")
			return site.Scope{}, false
		}
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
		return site.Scope{}, false
	}
	return scope, true
}

// validSiteID returns the site id of the path when it has the canonical UUID
// form. A malformed id is indistinguishable from an invisible site.
func validSiteID(request *http.Request) (string, bool) {
	siteID := chi.URLParam(request, "siteId")
	return siteID, siteIDPattern.MatchString(siteID)
}

func writeJSONResponse(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Vary", "Authorization")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

// mapSiteError translates a site service error into the public status. A
// site the caller cannot see and a malformed site id are both 404: the
// response never reveals which case it is.
func mapSiteError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, site.ErrSiteNotFound) || errors.Is(err, site.ErrInvalidSiteID):
		writeError(writer, request, http.StatusNotFound, "not_found", "the site was not found")
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
	}
}

func listSitesHandler(authorizer *authz.MembershipAuthorizer, service *site.Service) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		scope, ok := resolveScope(writer, request, authorizer)
		if !ok {
			return
		}
		if service == nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
			return
		}
		response, err := service.ListSites(request.Context(), scope)
		if err != nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
			return
		}
		writeJSONResponse(writer, http.StatusOK, response)
	})
}

func siteOverviewHandler(authorizer *authz.MembershipAuthorizer, service *site.Service) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		scope, ok := resolveScope(writer, request, authorizer)
		if !ok {
			return
		}
		siteID, ok := validSiteID(request)
		if !ok {
			writeError(writer, request, http.StatusNotFound, "not_found", "the site was not found")
			return
		}
		if service == nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
			return
		}
		response, err := service.GetOverview(request.Context(), scope, siteID)
		if err != nil {
			mapSiteError(writer, request, err)
			return
		}
		writeJSONResponse(writer, http.StatusOK, response)
	})
}

func listDevicesHandler(authorizer *authz.MembershipAuthorizer, service *site.Service) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		scope, ok := resolveScope(writer, request, authorizer)
		if !ok {
			return
		}
		siteID, ok := validSiteID(request)
		if !ok {
			writeError(writer, request, http.StatusNotFound, "not_found", "the site was not found")
			return
		}
		if service == nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
			return
		}
		response, err := service.ListDevices(request.Context(), scope, siteID)
		if err != nil {
			mapSiteError(writer, request, err)
			return
		}
		writeJSONResponse(writer, http.StatusOK, response)
	})
}
