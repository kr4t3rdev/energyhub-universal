package authz

import (
	"errors"
	"strings"

	"energyhub-universal/backend/internal/contracts"
	"energyhub-universal/backend/internal/site"
)

var (
	ErrUnauthenticated       = errors.New("principal is unauthenticated")
	ErrInvalidOrganization   = errors.New("organization id is invalid")
	ErrOrganizationForbidden = errors.New("principal is not a member of the organization")
)

// Principal is the result of a future token verifier.
// It contains no raw token and is not populated by this package.
type Principal struct {
	Subject       string
	Organizations []contracts.OrganizationSummary
}

// ScopeForOrganization converts verified membership into the tenant scope
// required by the site service. The organization path is never trusted alone.
func ScopeForOrganization(principal Principal, organizationID string) (site.Scope, error) {
	subject := strings.TrimSpace(principal.Subject)
	if subject == "" {
		return site.Scope{}, ErrUnauthenticated
	}

	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return site.Scope{}, ErrInvalidOrganization
	}

	for _, organization := range principal.Organizations {
		if strings.TrimSpace(organization.ID) == organizationID {
			return site.Scope{
				OrganizationID: organizationID,
				Subject:        subject,
			}, nil
		}
	}

	return site.Scope{}, ErrOrganizationForbidden
}
