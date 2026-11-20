package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
	"energyhub-universal/backend/internal/site"
)

type fakeSiteReader struct {
	subject        string
	organizationID string
	siteID         string
	sites          []contracts.SiteSummary
	overview       contracts.SiteOverview
	devices        []contracts.DeviceSummary
	err            error
	calls          int
}

func (f *fakeSiteReader) ListSites(_ context.Context, subject, organizationID string) ([]contracts.SiteSummary, error) {
	f.calls++
	f.subject = subject
	f.organizationID = organizationID
	return f.sites, f.err
}

func (f *fakeSiteReader) GetOverview(_ context.Context, subject, organizationID, siteID string) (contracts.SiteOverview, error) {
	f.calls++
	f.subject = subject
	f.organizationID = organizationID
	f.siteID = siteID
	return f.overview, f.err
}

func (f *fakeSiteReader) ListDevices(_ context.Context, subject, organizationID, siteID string) ([]contracts.DeviceSummary, error) {
	f.calls++
	f.subject = subject
	f.organizationID = organizationID
	f.siteID = siteID
	return f.devices, f.err
}

func sitesRouter(t *testing.T, reader *fakeSiteReader) http.Handler {
	t.Helper()
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	memberships := &fakeMemberships{organizations: []contracts.OrganizationSummary{
		{ID: "org-1", Name: "Demo Org", Role: contracts.RoleViewer},
	}}
	return NewRouter(
		WithAuthentication(verifier, memberships),
		WithSites(site.NewService(reader)),
	)
}

func sitesRequest(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer <redacted>")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestMeRouteReturnsCallerOrganizations(t *testing.T) {
	reader := &fakeSiteReader{}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/me")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body contracts.CurrentUser
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if body.Subject != "dev|user-1" || len(body.Organizations) != 1 || body.Organizations[0].ID != "org-1" {
		t.Fatalf("body = %#v, want the caller with org-1", body)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0: /v1/me reads memberships only", reader.calls)
	}
}

func TestOrganizationsRouteStillWorksWithSitesEnabled(t *testing.T) {
	reader := &fakeSiteReader{}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
}

func TestSitesRoutesAreNotRegisteredWithoutSitesOption(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	memberships := &fakeMemberships{}
	handler := NewRouter(WithAuthentication(verifier, memberships))

	for _, path := range []string{"/v1/me", "/v1/organizations/org-1/sites"} {
		response := sitesRequest(t, handler, path)
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s: status = %d, want 404", path, response.Code)
		}
	}

	response := sitesRequest(t, handler, "/v1/organizations")
	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/organizations: status = %d, want 200", response.Code)
	}
}

func TestSitesRouteRequiresAuthentication(t *testing.T) {
	reader := &fakeSiteReader{}
	response := httptest.NewRecorder()
	sitesRouter(t, reader).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/organizations/org-1/sites", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0", reader.calls)
	}
}

func TestSitesRouteReturnsForbiddenForNonMember(t *testing.T) {
	reader := &fakeSiteReader{
		sites: []contracts.SiteSummary{{ID: "site-1", Name: "Casa de ejemplo", Timezone: "Europe/Madrid"}},
	}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-2/sites")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", response.Code, response.Body.String())
	}
	var envelope contracts.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if envelope.Code != "forbidden" || envelope.RequestID == "" {
		t.Fatalf("envelope = %#v, want forbidden with request id", envelope)
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0: a non-member must not reach the reader", reader.calls)
	}
}

func TestSitesRouteListsSites(t *testing.T) {
	reader := &fakeSiteReader{
		sites: []contracts.SiteSummary{{ID: "11111111-1111-4111-8111-111111111111", Name: "Casa de ejemplo", Timezone: "Europe/Madrid"}},
	}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body contracts.SiteListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("items = %#v, want the single site", body.Items)
	}
	if reader.subject != "dev|user-1" || reader.organizationID != "org-1" {
		t.Fatalf("reader scope = (%q, %q), want (dev|user-1, org-1)", reader.subject, reader.organizationID)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Vary") != "Authorization" {
		t.Fatalf("headers = %v, want no-store and Vary Authorization", response.Header())
	}
}

func TestSitesRouteListsEmptySitesAsEmptyArray(t *testing.T) {
	reader := &fakeSiteReader{sites: []contracts.SiteSummary{}}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body contracts.SiteListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if body.Items == nil || len(body.Items) != 0 {
		t.Fatalf("items = %#v, want an empty non-nil array", body.Items)
	}
}

func TestOverviewRouteReturnsSiteOverview(t *testing.T) {
	reader := &fakeSiteReader{
		overview: contracts.SiteOverview{
			Site:      contracts.SiteSummary{ID: "11111111-1111-4111-8111-111111111111", Name: "Casa de ejemplo", Timezone: "Europe/Madrid"},
			Readings:  []contracts.ReadingSummary{},
			Devices:   []contracts.DeviceSummary{{ID: "22222222-2222-4222-8222-222222222222", Name: "Inversor solar", Kind: "inversor"}},
			UpdatedAt: nil,
		},
	}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites/11111111-1111-4111-8111-111111111111/overview")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body contracts.SiteOverview
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if body.Site.ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("site = %#v, want the site", body.Site)
	}
	if body.Readings == nil || len(body.Readings) != 0 {
		t.Fatalf("readings = %#v, want an empty non-nil array", body.Readings)
	}
	if body.UpdatedAt != nil {
		t.Fatalf("updatedAt = %v, want nil", body.UpdatedAt)
	}
	if len(body.Devices) != 1 || body.Devices[0].Name != "Inversor solar" {
		t.Fatalf("devices = %#v, want the single device", body.Devices)
	}
	if reader.siteID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("reader site = %q, want the path site", reader.siteID)
	}
}

func TestOverviewRouteReturnsNotFoundForInvisibleSite(t *testing.T) {
	reader := &fakeSiteReader{err: site.ErrSiteNotFound}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites/99999999-9999-4999-8999-999999999999/overview")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
	var envelope contracts.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if envelope.Code != "not_found" || envelope.RequestID == "" {
		t.Fatalf("envelope = %#v, want not_found with request id", envelope)
	}
}

func TestOverviewRouteReturnsNotFoundForMalformedSiteID(t *testing.T) {
	reader := &fakeSiteReader{}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites/not-a-uuid/overview")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0: a malformed id must not reach the reader", reader.calls)
	}
}

func TestOverviewRouteReturnsForbiddenForNonMemberBeforeNotFound(t *testing.T) {
	reader := &fakeSiteReader{err: site.ErrSiteNotFound}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-2/sites/99999999-9999-4999-8999-999999999999/overview")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: membership is checked before existence", response.Code)
	}
}

func TestDevicesRouteListsDevices(t *testing.T) {
	reader := &fakeSiteReader{
		devices: []contracts.DeviceSummary{{ID: "22222222-2222-4222-8222-222222222222", Name: "Inversor solar", Kind: "inversor"}},
	}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites/11111111-1111-4111-8111-111111111111/devices")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body contracts.DeviceListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if len(body.Items) != 1 || body.Items[0].Kind != "inversor" {
		t.Fatalf("items = %#v, want the single device", body.Items)
	}
}

func TestDevicesRouteReturnsNotFoundForInvisibleSite(t *testing.T) {
	reader := &fakeSiteReader{err: site.ErrSiteNotFound}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites/99999999-9999-4999-8999-999999999999/devices")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
}

func TestDevicesRouteReturnsNotFoundForMalformedSiteID(t *testing.T) {
	reader := &fakeSiteReader{}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites/not-a-uuid/devices")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0", reader.calls)
	}
}

func TestSitesRouteReturnsInternalErrorOnReaderFailure(t *testing.T) {
	reader := &fakeSiteReader{err: errors.New("database unavailable")}
	response := sitesRequest(t, sitesRouter(t, reader), "/v1/organizations/org-1/sites")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
	}
	var envelope contracts.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", response.Body.String(), err)
	}
	if envelope.Code != "internal_error" {
		t.Fatalf("envelope = %#v, want internal_error without details", envelope)
	}
}

func TestSitesRouteReturnsInternalErrorOnMembershipFailure(t *testing.T) {
	verifier := &fakeVerifier{principal: authz.Principal{Subject: "dev|user-1"}}
	memberships := &fakeMemberships{err: errors.New("database unavailable")}
	reader := &fakeSiteReader{}
	handler := NewRouter(
		WithAuthentication(verifier, memberships),
		WithSites(site.NewService(reader)),
	)
	response := sitesRequest(t, handler, "/v1/organizations/org-1/sites")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
	}
	if reader.calls != 0 {
		t.Fatalf("reader calls = %d, want 0", reader.calls)
	}
}
