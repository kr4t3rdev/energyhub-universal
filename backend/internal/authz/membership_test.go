package authz

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

type fakeMembershipReader struct {
	subject string
	orgs    []contracts.OrganizationSummary
	err     error
	calls   int
}

func (f *fakeMembershipReader) ListOrganizations(_ context.Context, subject string) ([]contracts.OrganizationSummary, error) {
	f.calls++
	f.subject = subject
	return f.orgs, f.err
}

func TestMembershipAuthorizerLoadsOrganizationsForVerifiedSubject(t *testing.T) {
	reader := &fakeMembershipReader{
		orgs: []contracts.OrganizationSummary{
			{ID: "org-1", Name: "Demo", Role: contracts.RoleViewer},
		},
	}
	authorizer := NewMembershipAuthorizer(reader)

	organizations, err := authorizer.Organizations(context.Background(), Principal{Subject: "auth0|user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if reader.subject != "auth0|user-1" || reader.calls != 1 {
		t.Fatalf("reader = %#v, want one call for auth0|user-1", reader)
	}
	if len(organizations) != 1 || organizations[0].ID != "org-1" {
		t.Fatalf("organizations = %#v, want org-1", organizations)
	}
}

func TestMembershipAuthorizerCreatesScopeFromServerMembership(t *testing.T) {
	authorizer := NewMembershipAuthorizer(&fakeMembershipReader{
		orgs: []contracts.OrganizationSummary{{ID: "org-1"}},
	})

	scope, err := authorizer.ScopeForOrganization(
		context.Background(),
		Principal{Subject: "auth0|user-1"},
		"org-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if scope.Subject != "auth0|user-1" || scope.OrganizationID != "org-1" {
		t.Fatalf("scope = %#v, want auth0|user-1/org-1", scope)
	}
}

func TestMembershipAuthorizerRejectsOrganizationOutsideServerMembership(t *testing.T) {
	authorizer := NewMembershipAuthorizer(&fakeMembershipReader{
		orgs: []contracts.OrganizationSummary{{ID: "org-1"}},
	})

	_, err := authorizer.ScopeForOrganization(
		context.Background(),
		Principal{Subject: "auth0|user-1"},
		"org-2",
	)
	if !errors.Is(err, ErrOrganizationForbidden) {
		t.Fatalf("err = %v, want ErrOrganizationForbidden", err)
	}
}

func TestMembershipAuthorizerRejectsUnauthenticatedPrincipalBeforeReader(t *testing.T) {
	reader := &fakeMembershipReader{}
	authorizer := NewMembershipAuthorizer(reader)

	_, err := authorizer.Organizations(context.Background(), Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0", reader.calls)
	}
}

func TestMembershipAuthorizerPropagatesReaderError(t *testing.T) {
	want := errors.New("membership store unavailable")
	authorizer := NewMembershipAuthorizer(&fakeMembershipReader{err: want})

	_, err := authorizer.Organizations(context.Background(), Principal{Subject: "auth0|user-1"})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMembershipAuthorizerScopePropagatesReaderError(t *testing.T) {
	want := errors.New("membership store unavailable")
	authorizer := NewMembershipAuthorizer(&fakeMembershipReader{err: want})

	_, err := authorizer.ScopeForOrganization(
		context.Background(),
		Principal{Subject: "auth0|user-1"},
		"org-1",
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestMembershipAuthorizerRejectsUnavailableReader(t *testing.T) {
	authorizer := NewMembershipAuthorizer(nil)

	_, err := authorizer.Organizations(context.Background(), Principal{Subject: "auth0|user-1"})
	if !errors.Is(err, ErrMembershipReaderUnavailable) {
		t.Fatalf("err = %v, want ErrMembershipReaderUnavailable", err)
	}
}
