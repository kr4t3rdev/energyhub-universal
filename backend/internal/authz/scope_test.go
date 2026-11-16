package authz

import (
	"errors"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

func TestScopeForOrganizationAllowsKnownMembership(t *testing.T) {
	principal := Principal{
		Subject: "user-1",
		Organizations: []contracts.OrganizationSummary{
			{ID: "org-1", Name: "Demo", Role: contracts.RoleViewer},
		},
	}

	scope, err := ScopeForOrganization(principal, "org-1")
	if err != nil {
		t.Fatal(err)
	}
	if scope.Subject != "user-1" || scope.OrganizationID != "org-1" {
		t.Fatalf("scope = %#v, want user-1/org-1", scope)
	}
}

func TestScopeForOrganizationRejectsUnknownMembership(t *testing.T) {
	principal := Principal{
		Subject:       "user-1",
		Organizations: []contracts.OrganizationSummary{{ID: "org-1"}},
	}

	_, err := ScopeForOrganization(principal, "org-2")
	if !errors.Is(err, ErrOrganizationForbidden) {
		t.Fatalf("err = %v, want ErrOrganizationForbidden", err)
	}
}

func TestScopeForOrganizationRejectsMissingSubject(t *testing.T) {
	_, err := ScopeForOrganization(Principal{}, "org-1")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestScopeForOrganizationRejectsEmptyOrganization(t *testing.T) {
	principal := Principal{Subject: "user-1"}

	_, err := ScopeForOrganization(principal, " ")
	if !errors.Is(err, ErrInvalidOrganization) {
		t.Fatalf("err = %v, want ErrInvalidOrganization", err)
	}
}
