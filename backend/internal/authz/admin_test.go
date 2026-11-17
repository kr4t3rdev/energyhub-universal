package authz

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

type fakeMembershipWriter struct {
	grantCalls  int
	revokeCalls int
	admin       string
	org         string
	subject     string
	role        contracts.OrganizationRole
	err         error
}

func (f *fakeMembershipWriter) Grant(_ context.Context, adminSubject, organizationID, subject string, role contracts.OrganizationRole) error {
	f.grantCalls++
	f.admin = adminSubject
	f.org = organizationID
	f.subject = subject
	f.role = role
	return f.err
}

func (f *fakeMembershipWriter) Revoke(_ context.Context, adminSubject, organizationID, subject string) error {
	f.revokeCalls++
	f.admin = adminSubject
	f.org = organizationID
	f.subject = subject
	return f.err
}

func TestAdminAuthorizerAcceptsConfiguredSubject(t *testing.T) {
	authorizer := NewAdminAuthorizer(" auth0|admin ")

	if err := authorizer.Authorize(Principal{Subject: " auth0|admin "}); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestAdminAuthorizerRejectsOtherSubject(t *testing.T) {
	authorizer := NewAdminAuthorizer("auth0|admin")

	err := authorizer.Authorize(Principal{Subject: "auth0|user-1"})
	if !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("err = %v, want ErrAdminForbidden", err)
	}
}

func TestAdminAuthorizerRejectsMissingPrincipal(t *testing.T) {
	authorizer := NewAdminAuthorizer("auth0|admin")

	if err := authorizer.Authorize(Principal{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestAdminAuthorizerFailsClosedWithoutConfiguration(t *testing.T) {
	if err := NewAdminAuthorizer("  ").Authorize(Principal{Subject: "auth0|admin"}); !errors.Is(err, ErrAdminNotConfigured) {
		t.Fatalf("err = %v, want ErrAdminNotConfigured", err)
	}

	var authorizer *AdminAuthorizer
	if err := authorizer.Authorize(Principal{Subject: "auth0|admin"}); !errors.Is(err, ErrAdminNotConfigured) {
		t.Fatalf("nil authorizer err = %v, want ErrAdminNotConfigured", err)
	}
}

type fakeOrganizationWriter struct {
	createCalls int
	admin       string
	org         string
	name        string
	err         error
}

func (f *fakeOrganizationWriter) Create(_ context.Context, adminSubject, organizationID, name string) error {
	f.createCalls++
	f.admin = adminSubject
	f.org = organizationID
	f.name = name
	return f.err
}

func TestAdminOrganizationServiceCreatesAsConfiguredAdmin(t *testing.T) {
	writer := &fakeOrganizationWriter{}
	service := NewAdminOrganizationService("auth0|admin", writer)

	err := service.Create(context.Background(), Principal{Subject: " auth0|admin "}, " org-1 ", " Demo Org ")
	if err != nil {
		t.Fatal(err)
	}
	if writer.createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", writer.createCalls)
	}
	if writer.admin != "auth0|admin" || writer.org != "org-1" || writer.name != "Demo Org" {
		t.Fatalf("writer call = %#v", writer)
	}
}

func TestAdminOrganizationServiceRejectsNonAdminBeforeWriter(t *testing.T) {
	writer := &fakeOrganizationWriter{}
	service := NewAdminOrganizationService("auth0|admin", writer)

	err := service.Create(context.Background(), Principal{Subject: "auth0|other"}, "org-1", "Demo Org")
	if !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("err = %v, want ErrAdminForbidden", err)
	}
	if writer.createCalls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.createCalls)
	}
}

func TestAdminOrganizationServiceRejectsInvalidTargetBeforeWriter(t *testing.T) {
	tests := []struct {
		name    string
		org     string
		orgName string
	}{
		{name: "missing organization", org: " ", orgName: "Demo Org"},
		{name: "missing name", org: "org-1", orgName: " "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer := &fakeOrganizationWriter{}
			service := NewAdminOrganizationService("auth0|admin", writer)

			err := service.Create(context.Background(), Principal{Subject: "auth0|admin"}, test.org, test.orgName)
			if !errors.Is(err, ErrInvalidOrganizationTarget) {
				t.Fatalf("err = %v, want ErrInvalidOrganizationTarget", err)
			}
			if writer.createCalls != 0 {
				t.Fatalf("writer calls = %d, want 0", writer.createCalls)
			}
		})
	}
}

func TestAdminOrganizationServicePropagatesWriterError(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewAdminOrganizationService("auth0|admin", &fakeOrganizationWriter{err: want})

	err := service.Create(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "Demo Org")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestAdminOrganizationServiceRejectsUnavailableWriter(t *testing.T) {
	service := NewAdminOrganizationService("auth0|admin", nil)

	err := service.Create(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "Demo Org")
	if !errors.Is(err, ErrOrganizationWriterUnavailable) {
		t.Fatalf("err = %v, want ErrOrganizationWriterUnavailable", err)
	}
}

func TestAdminOrganizationServiceFailsClosedWithoutConfiguration(t *testing.T) {
	writer := &fakeOrganizationWriter{}
	service := NewAdminOrganizationService(" ", writer)

	err := service.Create(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "Demo Org")
	if !errors.Is(err, ErrAdminNotConfigured) {
		t.Fatalf("err = %v, want ErrAdminNotConfigured", err)
	}
	if writer.createCalls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.createCalls)
	}
}

func TestAdminOrganizationServiceRejectsNilService(t *testing.T) {
	var service *AdminOrganizationService

	if err := service.Create(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "Demo Org"); !errors.Is(err, ErrAdminNotConfigured) {
		t.Fatalf("err = %v, want ErrAdminNotConfigured", err)
	}
}

func TestAdminMembershipServiceGrantsAsConfiguredAdmin(t *testing.T) {
	writer := &fakeMembershipWriter{}
	service := NewAdminMembershipService("auth0|admin", writer)

	err := service.Grant(
		context.Background(),
		Principal{Subject: " auth0|admin "},
		" org-1 ",
		" auth0|user-1 ",
		contracts.RoleViewer,
	)
	if err != nil {
		t.Fatal(err)
	}
	if writer.grantCalls != 1 {
		t.Fatalf("grant calls = %d, want 1", writer.grantCalls)
	}
	if writer.admin != "auth0|admin" || writer.org != "org-1" || writer.subject != "auth0|user-1" || writer.role != contracts.RoleViewer {
		t.Fatalf("writer call = %#v", writer)
	}
}

func TestAdminMembershipServiceRevokesAsConfiguredAdmin(t *testing.T) {
	writer := &fakeMembershipWriter{}
	service := NewAdminMembershipService("auth0|admin", writer)

	err := service.Revoke(
		context.Background(),
		Principal{Subject: "auth0|admin"},
		"org-1",
		"auth0|user-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if writer.revokeCalls != 1 || writer.admin != "auth0|admin" || writer.org != "org-1" || writer.subject != "auth0|user-1" {
		t.Fatalf("writer call = %#v", writer)
	}
}

func TestAdminMembershipServiceRejectsNonAdminBeforeWriter(t *testing.T) {
	writer := &fakeMembershipWriter{}
	service := NewAdminMembershipService("auth0|admin", writer)

	err := service.Grant(
		context.Background(),
		Principal{Subject: "auth0|user-1"},
		"org-1",
		"auth0|user-2",
		contracts.RoleOwner,
	)
	if !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("err = %v, want ErrAdminForbidden", err)
	}
	if writer.grantCalls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.grantCalls)
	}
}

func TestAdminMembershipServiceRejectsInvalidTargetBeforeWriter(t *testing.T) {
	tests := []struct {
		name    string
		org     string
		subject string
		role    contracts.OrganizationRole
	}{
		{name: "missing organization", org: " ", subject: "auth0|user-1", role: contracts.RoleViewer},
		{name: "missing subject", org: "org-1", subject: " ", role: contracts.RoleViewer},
		{name: "invalid role", org: "org-1", subject: "auth0|user-1", role: contracts.OrganizationRole("superuser")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer := &fakeMembershipWriter{}
			service := NewAdminMembershipService("auth0|admin", writer)

			err := service.Grant(
				context.Background(),
				Principal{Subject: "auth0|admin"},
				test.org,
				test.subject,
				test.role,
			)
			if !errors.Is(err, ErrInvalidMembershipTarget) {
				t.Fatalf("err = %v, want ErrInvalidMembershipTarget", err)
			}
			if writer.grantCalls != 0 {
				t.Fatalf("writer calls = %d, want 0", writer.grantCalls)
			}
		})
	}
}

func TestAdminMembershipServiceRevokeRejectsInvalidTarget(t *testing.T) {
	writer := &fakeMembershipWriter{}
	service := NewAdminMembershipService("auth0|admin", writer)

	if err := service.Revoke(context.Background(), Principal{Subject: "auth0|admin"}, "", "auth0|user-1"); !errors.Is(err, ErrInvalidMembershipTarget) {
		t.Fatalf("err = %v, want ErrInvalidMembershipTarget", err)
	}
	if writer.revokeCalls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.revokeCalls)
	}
}

func TestAdminMembershipServicePropagatesWriterError(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewAdminMembershipService("auth0|admin", &fakeMembershipWriter{err: want})

	err := service.Grant(
		context.Background(),
		Principal{Subject: "auth0|admin"},
		"org-1",
		"auth0|user-1",
		contracts.RoleMember,
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestAdminMembershipServiceRejectsUnavailableWriter(t *testing.T) {
	service := NewAdminMembershipService("auth0|admin", nil)

	err := service.Grant(
		context.Background(),
		Principal{Subject: "auth0|admin"},
		"org-1",
		"auth0|user-1",
		contracts.RoleMember,
	)
	if !errors.Is(err, ErrMembershipWriterUnavailable) {
		t.Fatalf("err = %v, want ErrMembershipWriterUnavailable", err)
	}
}

func TestAdminMembershipServiceRejectsNilService(t *testing.T) {
	var service *AdminMembershipService

	if err := service.Grant(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "auth0|user-1", contracts.RoleMember); !errors.Is(err, ErrAdminNotConfigured) {
		t.Fatalf("grant err = %v, want ErrAdminNotConfigured", err)
	}
	if err := service.Revoke(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "auth0|user-1"); !errors.Is(err, ErrAdminNotConfigured) {
		t.Fatalf("revoke err = %v, want ErrAdminNotConfigured", err)
	}
}

func TestAdminMembershipServiceAcceptsEveryContractRole(t *testing.T) {
	for _, role := range []contracts.OrganizationRole{
		contracts.RoleOwner,
		contracts.RoleAdmin,
		contracts.RoleMember,
		contracts.RoleViewer,
	} {
		writer := &fakeMembershipWriter{}
		service := NewAdminMembershipService("auth0|admin", writer)

		if err := service.Grant(context.Background(), Principal{Subject: "auth0|admin"}, "org-1", "auth0|user-1", role); err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		if writer.role != role {
			t.Fatalf("writer role = %q, want %q", writer.role, role)
		}
	}
}
