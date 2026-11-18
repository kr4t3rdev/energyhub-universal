package httpapi

import (
	"encoding/json"
	"net/http"

	"energyhub-universal/backend/internal/authn"
	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

// organizationsHandler returns the organizations the verified principal belongs
// to. The subject comes from the verified token only: the request never chooses
// which tenant it wants.
func organizationsHandler(memberships authz.MembershipReader) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := authn.PrincipalFromContext(request.Context())
		if !ok {
			writeError(writer, request, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}
		if memberships == nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
			return
		}

		organizations, err := memberships.ListOrganizations(request.Context(), principal.Subject)
		if err != nil {
			writeError(writer, request, http.StatusInternalServerError, "internal_error", "the request could not be completed")
			return
		}
		if organizations == nil {
			organizations = []contracts.OrganizationSummary{}
		}

		writer.Header().Set("Content-Type", "application/json")
		// The response depends on the verified subject, so no shared cache may
		// keep it.
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Vary", "Authorization")
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(contracts.CurrentUser{
			Subject:       principal.Subject,
			Organizations: organizations,
		})
	})
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	requestID, _ := RequestIDFromContext(request.Context())
	WriteErrorResponse(writer, status, code, message, requestID)
}
