package site

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/contracts"
)

type fakeReader struct {
	listSitesSubject        string
	listSitesOrganizationID string
	overviewSubject         string
	overviewOrganizationID  string
	overviewSiteID          string
	devicesSubject          string
	listSitesResult         []contracts.SiteSummary
	overviewResult          contracts.SiteOverview
	err                     error
	calls                   int
}

func (f *fakeReader) ListSites(_ context.Context, subject, organizationID string) ([]contracts.SiteSummary, error) {
	f.calls++
	f.listSitesSubject = subject
	f.listSitesOrganizationID = organizationID
	return f.listSitesResult, f.err
}

func (f *fakeReader) GetOverview(_ context.Context, subject, organizationID, siteID string) (contracts.SiteOverview, error) {
	f.calls++
	f.overviewSubject = subject
	f.overviewOrganizationID = organizationID
	f.overviewSiteID = siteID
	return f.overviewResult, f.err
}

func (f *fakeReader) ListDevices(_ context.Context, subject, _, _ string) ([]contracts.DeviceSummary, error) {
	f.calls++
	f.devicesSubject = subject
	return nil, f.err
}

func TestServiceListSitesRequiresAuthenticatedOrganizationScope(t *testing.T) {
	reader := &fakeReader{
		listSitesResult: []contracts.SiteSummary{{ID: "site-1", Name: "Casa de ejemplo"}},
	}
	service := NewService(reader)
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	response, err := service.ListSites(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if reader.listSitesSubject != scope.Subject {
		t.Fatalf("subject = %q, want %q", reader.listSitesSubject, scope.Subject)
	}
	if reader.listSitesOrganizationID != scope.OrganizationID {
		t.Fatalf("organizationID = %q, want %q", reader.listSitesOrganizationID, scope.OrganizationID)
	}
	if len(response.Items) != 1 || response.Items[0].ID != "site-1" {
		t.Fatalf("items = %#v, want site-1", response.Items)
	}
}

func TestServiceRejectsIncompleteScopeBeforeReaderCall(t *testing.T) {
	reader := &fakeReader{}
	service := NewService(reader)

	_, err := service.ListSites(context.Background(), Scope{OrganizationID: "org-1"})
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("err = %v, want ErrInvalidScope", err)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0", reader.calls)
	}
}

func TestServiceOverviewPassesOrganizationAndSiteScope(t *testing.T) {
	reader := &fakeReader{
		overviewResult: contracts.SiteOverview{
			Site: contracts.SiteSummary{ID: "site-1", Name: "Casa de ejemplo"},
		},
	}
	service := NewService(reader)
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	response, err := service.GetOverview(context.Background(), scope, "site-1")
	if err != nil {
		t.Fatal(err)
	}
	if reader.overviewSubject != "user-1" {
		t.Fatalf("subject = %q, want %q", reader.overviewSubject, "user-1")
	}
	if reader.overviewOrganizationID != "org-1" || reader.overviewSiteID != "site-1" {
		t.Fatalf("scope = (%q, %q), want (org-1, site-1)", reader.overviewOrganizationID, reader.overviewSiteID)
	}
	if response.Site.ID != "site-1" {
		t.Fatalf("site = %#v, want site-1", response.Site)
	}
}

func TestServiceListDevicesPassesOrganizationAndSiteScope(t *testing.T) {
	reader := &fakeReader{}
	service := NewService(reader)
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	response, err := service.ListDevices(context.Background(), scope, "site-1")
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 {
		t.Fatalf("reader calls = %d, want 1", reader.calls)
	}
	if response.Items != nil {
		t.Fatalf("items = %#v, want nil", response.Items)
	}
}

func TestServiceRejectsEmptySiteIDBeforeReaderCall(t *testing.T) {
	reader := &fakeReader{}
	service := NewService(reader)
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	_, err := service.GetOverview(context.Background(), scope, " ")
	if !errors.Is(err, ErrInvalidSiteID) {
		t.Fatalf("err = %v, want ErrInvalidSiteID", err)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0", reader.calls)
	}
}

func TestServicePropagatesReaderError(t *testing.T) {
	want := errors.New("reader unavailable")
	service := NewService(&fakeReader{err: want})
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	_, err := service.ListSites(context.Background(), scope)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestServiceRejectsUnavailableReader(t *testing.T) {
	service := NewService(nil)
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	_, err := service.ListSites(context.Background(), scope)
	if !errors.Is(err, ErrReaderUnavailable) {
		t.Fatalf("err = %v, want ErrReaderUnavailable", err)
	}
}

func TestServicePassesVerifiedSubjectToReaderOnListDevices(t *testing.T) {
	reader := &fakeReader{}
	service := NewService(reader)
	scope := Scope{OrganizationID: "org-1", Subject: "user-1"}

	if _, err := service.ListDevices(context.Background(), scope, "site-1"); err != nil {
		t.Fatal(err)
	}
	if reader.devicesSubject != "user-1" {
		t.Fatalf("subject = %q, want %q", reader.devicesSubject, "user-1")
	}
}
