package authz

import (
	"context"
	"errors"

	"energyhub-universal/backend/internal/contracts"
	"energyhub-universal/backend/internal/site"
)

var ErrMembershipReaderUnavailable = errors.New("membership reader unavailable")

// MembershipReader resolves organization membership from a trusted server-side
// source. Implementations must scope every lookup by the verified subject.
type MembershipReader interface {
	ListOrganizations(ctx context.Context, subject string) ([]contracts.OrganizationSummary, error)
}

type MembershipAuthorizer struct {
	reader MembershipReader
}

func NewMembershipAuthorizer(reader MembershipReader) *MembershipAuthorizer {
	return &MembershipAuthorizer{reader: reader}
}

func (a *MembershipAuthorizer) Organizations(ctx context.Context, principal Principal) ([]contracts.OrganizationSummary, error) {
	if err := validatePrincipal(principal); err != nil {
		return nil, err
	}
	if a == nil || a.reader == nil {
		return nil, ErrMembershipReaderUnavailable
	}
	return a.reader.ListOrganizations(ctx, principal.Subject)
}

func (a *MembershipAuthorizer) ScopeForOrganization(
	ctx context.Context,
	principal Principal,
	organizationID string,
) (site.Scope, error) {
	organizations, err := a.Organizations(ctx, principal)
	if err != nil {
		return site.Scope{}, err
	}
	return ScopeForOrganization(
		Principal{
			Subject:       principal.Subject,
			Organizations: organizations,
		},
		organizationID,
	)
}

func validatePrincipal(principal Principal) error {
	if principal.Subject == "" {
		return ErrUnauthenticated
	}
	return nil
}
