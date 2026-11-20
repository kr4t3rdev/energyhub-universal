package postgres

import (
	"context"
	"errors"
	"testing"

	"energyhub-universal/backend/internal/site"
)

// TestIntegrationSiteTenantIsolationAndAdminWrites proves the sites boundary
// against a real server: sites and devices are created only through the
// audited admin functions, the runtime credential reads only the sites of
// the organizations its subject belongs to, and it cannot write the tenant
// tables or call the admin functions directly.
func TestIntegrationSiteTenantIsolationAndAdminWrites(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	environment := newIntegrationEnvironment(t, superuserURL)
	ctx := context.Background()

	const (
		siteOneID   = "11111111-1111-4111-8111-111111111111"
		siteTwoID   = "22222222-2222-4222-8222-222222222222"
		deviceOneID = "33333333-3333-4333-8333-333333333333"
		deviceTwoID = "44444444-4444-4444-8444-444444444444"
	)

	t.Run("admin provisions sites and devices", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-1', 'Demo Org')"); err != nil {
			t.Fatalf("create organization: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-2', 'Other Org')"); err != nil {
			t.Fatalf("create organization: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-1', 'dev|user-a', 'viewer')"); err != nil {
			t.Fatalf("grant membership: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "SELECT app.grant_membership('org-2', 'dev|user-b', 'viewer')"); err != nil {
			t.Fatalf("grant membership: %v", err)
		}

		writer := NewSiteWriter(NewDB(adminPool), "dev|admin")
		if err := writer.CreateSite(ctx, "dev|admin", "org-1", siteOneID, "HQ", "Europe/Madrid"); err != nil {
			t.Fatalf("create site: %v", err)
		}
		if err := writer.CreateSite(ctx, "dev|admin", "org-2", siteTwoID, "Branch", "Europe/Madrid"); err != nil {
			t.Fatalf("create site: %v", err)
		}
		if err := writer.CreateDevice(ctx, "dev|admin", "org-1", siteOneID, deviceOneID, "Main Meter", "meter"); err != nil {
			t.Fatalf("create device: %v", err)
		}
		if err := writer.CreateDevice(ctx, "dev|admin", "org-2", siteTwoID, deviceTwoID, "Branch Meter", "meter"); err != nil {
			t.Fatalf("create device: %v", err)
		}
	})

	t.Run("the admin writes are audited", func(t *testing.T) {
		privilegedPool := connect(t, withDatabase(superuserURL, environment.databaseName))
		defer privilegedPool.Close()

		for _, action := range []string{"create_site", "create_site_device"} {
			var audited int
			if err := privilegedPool.QueryRow(ctx,
				"SELECT count(*) FROM app.admin_audit_log WHERE operation = $1", action).Scan(&audited); err != nil {
				t.Fatalf("read the audit trail: %v", err)
			}
			if audited == 0 {
				t.Fatalf("no audit row for %s", action)
			}
		}
	})

	t.Run("runtime reads only its own sites", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		reader := NewSiteReader(NewDB(runtimePool))
		sites, err := reader.ListSites(ctx, "dev|user-a", "org-1")
		if err != nil {
			t.Fatalf("list sites: %v", err)
		}
		if len(sites) != 1 || sites[0].ID != siteOneID || sites[0].Name != "HQ" || sites[0].Timezone != "Europe/Madrid" {
			t.Fatalf("sites = %#v, want only the HQ site of org-1", sites)
		}

		others, err := reader.ListSites(ctx, "dev|user-b", "org-1")
		if err != nil {
			t.Fatalf("list sites: %v", err)
		}
		if len(others) != 0 {
			t.Fatalf("sites = %#v, want none for a subject outside org-1", others)
		}
	})

	t.Run("runtime overview has no readings yet", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		reader := NewSiteReader(NewDB(runtimePool))
		overview, err := reader.GetOverview(ctx, "dev|user-a", "org-1", siteOneID)
		if err != nil {
			t.Fatalf("get overview: %v", err)
		}
		if overview.Site.ID != siteOneID || overview.Site.Name != "HQ" {
			t.Fatalf("overview site = %#v, want the HQ site", overview.Site)
		}
		if overview.Readings == nil || len(overview.Readings) != 0 {
			t.Fatalf("readings = %#v, want an empty non-nil slice", overview.Readings)
		}
		if overview.UpdatedAt != nil {
			t.Fatalf("updatedAt = %v, want nil until telemetry exists", overview.UpdatedAt)
		}
	})

	t.Run("runtime cannot see another organization site", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		reader := NewSiteReader(NewDB(runtimePool))
		if _, err := reader.GetOverview(ctx, "dev|user-a", "org-1", siteTwoID); !errors.Is(err, site.ErrSiteNotFound) {
			t.Fatalf("overview err = %v, want ErrSiteNotFound", err)
		}
		if _, err := reader.ListDevices(ctx, "dev|user-a", "org-1", siteTwoID); !errors.Is(err, site.ErrSiteNotFound) {
			t.Fatalf("devices err = %v, want ErrSiteNotFound", err)
		}
		if _, err := reader.GetOverview(ctx, "dev|user-a", "org-1", "99999999-9999-4999-8999-999999999999"); !errors.Is(err, site.ErrSiteNotFound) {
			t.Fatalf("overview err = %v, want ErrSiteNotFound", err)
		}
		if _, err := reader.GetOverview(ctx, "dev|user-a", "org-1", "not-a-uuid"); !errors.Is(err, site.ErrSiteNotFound) {
			t.Fatalf("overview err = %v, want ErrSiteNotFound", err)
		}
	})

	t.Run("runtime lists only its own devices", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		reader := NewSiteReader(NewDB(runtimePool))
		devices, err := reader.ListDevices(ctx, "dev|user-a", "org-1", siteOneID)
		if err != nil {
			t.Fatalf("list devices: %v", err)
		}
		if len(devices) != 1 || devices[0].ID != deviceOneID || devices[0].Name != "Main Meter" || devices[0].Kind != "meter" {
			t.Fatalf("devices = %#v, want only the main meter", devices)
		}
	})

	t.Run("runtime cannot write sites directly", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		_, err := runtimePool.Exec(ctx, "INSERT INTO public.sites (id, organization_id, name, timezone) VALUES ('55555555-5555-4555-8555-555555555555', 'org-1', 'Direct', 'Europe/Madrid')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)

		_, err = runtimePool.Exec(ctx, "INSERT INTO public.site_devices (id, site_id, name, kind) VALUES ('66666666-6666-4666-8666-666666666666', '11111111-1111-4111-8111-111111111111', 'Direct', 'meter')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)
	})

	t.Run("runtime cannot call the admin site functions", func(t *testing.T) {
		runtimePool := connect(t, environment.runtimeURL)
		defer runtimePool.Close()

		_, err := runtimePool.Exec(ctx, "SELECT app.create_site('org-1', '77777777-7777-4777-8777-777777777777', 'Direct', 'Europe/Madrid')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)

		_, err = runtimePool.Exec(ctx, "SELECT app.create_site_device('org-1', '11111111-1111-4111-8111-111111111111', '88888888-8888-4888-8888-888888888888', 'Direct', 'meter')")
		if err == nil {
			t.Fatal("err = nil, want permission denied")
		}
		assertPermissionDenied(t, err)
	})

	t.Run("admin cannot read the tenant site tables", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM public.sites"); err == nil {
			t.Fatal("the administrative credential can read the sites table")
		} else {
			assertPermissionDenied(t, err)
		}
		if _, err := adminPool.Exec(ctx, "SELECT count(*) FROM public.site_devices"); err == nil {
			t.Fatal("the administrative credential can read the site devices table")
		} else {
			assertPermissionDenied(t, err)
		}
	})

	t.Run("deleting an organization cascades to its sites and devices", func(t *testing.T) {
		adminPool := connect(t, environment.adminURL)
		defer adminPool.Close()

		writer := NewSiteWriter(NewDB(adminPool), "dev|admin")
		if _, err := adminPool.Exec(ctx, "SELECT app.create_organization('org-3', 'Cascade Org')"); err != nil {
			t.Fatalf("create organization: %v", err)
		}
		const cascadeSiteID = "99999999-9999-4999-8999-999999999999"
		const cascadeDeviceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		if err := writer.CreateSite(ctx, "dev|admin", "org-3", cascadeSiteID, "Doomed", "Europe/Madrid"); err != nil {
			t.Fatalf("create site: %v", err)
		}
		if err := writer.CreateDevice(ctx, "dev|admin", "org-3", cascadeSiteID, cascadeDeviceID, "Doomed Meter", "meter"); err != nil {
			t.Fatalf("create device: %v", err)
		}

		privilegedPool := connect(t, withDatabase(superuserURL, environment.databaseName))
		defer privilegedPool.Close()
		if _, err := privilegedPool.Exec(ctx, "DELETE FROM public.organizations WHERE id = 'org-3'"); err != nil {
			t.Fatalf("delete organization: %v", err)
		}
		var remaining int
		if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM public.sites WHERE organization_id = 'org-3'").Scan(&remaining); err != nil {
			t.Fatalf("count sites: %v", err)
		}
		if remaining != 0 {
			t.Fatalf("sites remaining = %d, want 0", remaining)
		}
		if err := privilegedPool.QueryRow(ctx, "SELECT count(*) FROM public.site_devices WHERE site_id = $1", cascadeSiteID).Scan(&remaining); err != nil {
			t.Fatalf("count devices: %v", err)
		}
		if remaining != 0 {
			t.Fatalf("devices remaining = %d, want 0", remaining)
		}
	})
}
