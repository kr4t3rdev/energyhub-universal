package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
	"energyhub-universal/backend/internal/site"
)

const (
	listSitesQuery = `
SELECT s.id, s.name, s.timezone, s.organization_id
FROM public.sites AS s
ORDER BY s.name, s.id`

	getSiteQuery = listSitesQuery

	listSiteDevicesQuery = `
SELECT d.id, d.name, d.kind, d.status, d.site_id
FROM public.site_devices AS d
ORDER BY d.name, d.id`
)

// SiteReader reads sites and devices from PostgreSQL. Every read runs in a
// transaction that sets app.current_subject to the verified subject so RLS
// policies enforce the same subject even if a future query forgets the WHERE
// clause. The subject is the verified subject the service received in its
// scope, never a value the client chose.
type SiteReader struct {
	db DB
}

func NewSiteReader(db DB) *SiteReader {
	return &SiteReader{db: db}
}

func (r *SiteReader) ListSites(ctx context.Context, subject, organizationID string) ([]contracts.SiteSummary, error) {
	subject, organizationID, err := r.validate(subject, organizationID)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Set the tenant context so RLS policies filter rows by subject.
	if err := tx.Exec(ctx, setMembershipSubjectQuery, subject); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	rows, err := tx.Query(ctx, listSitesQuery)
	if err != nil {
		return nil, err
	}
	sites, err := scanSites(rows)
	if err != nil {
		return nil, err
	}
	filtered := make([]contracts.SiteSummary, 0)
	for _, s := range sites {
		if s.OrganizationID == organizationID {
			filtered = append(filtered, s)
		}
	}
	sites = filtered

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true
	return sites, nil
}

func (r *SiteReader) GetOverview(ctx context.Context, subject, organizationID, siteID string) (contracts.SiteOverview, error) {
	subject, organizationID, siteID, err := r.validateSite(subject, organizationID, siteID)
	if err != nil {
		return contracts.SiteOverview{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return contracts.SiteOverview{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Set the tenant context so RLS policies filter rows by subject.
	if err := tx.Exec(ctx, setMembershipSubjectQuery, subject); err != nil {
		_ = tx.Rollback(ctx)
		return contracts.SiteOverview{}, err
	}

	summary, err := r.readSite(ctx, tx, organizationID, siteID)
	if err != nil {
		return contracts.SiteOverview{}, err
	}
	devices, err := r.readDevices(ctx, tx, siteID)
	if err != nil {
		return contracts.SiteOverview{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return contracts.SiteOverview{}, err
	}
	committed = true
	// There is no telemetry source yet: the overview reports no readings and
	// no update time instead of inventing them.
	return contracts.SiteOverview{
		Site:      summary,
		Readings:  []contracts.ReadingSummary{},
		Devices:   devices,
		UpdatedAt: nil,
	}, nil
}

func (r *SiteReader) ListDevices(ctx context.Context, subject, organizationID, siteID string) ([]contracts.DeviceSummary, error) {
	subject, organizationID, siteID, err := r.validateSite(subject, organizationID, siteID)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Set the tenant context so RLS policies filter rows by subject.
	if err := tx.Exec(ctx, setMembershipSubjectQuery, subject); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	if _, err := r.readSite(ctx, tx, organizationID, siteID); err != nil {
		return nil, err
	}
	devices, err := r.readDevices(ctx, tx, siteID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true
	return devices, nil
}

// readSite returns the site when it belongs to the organization and is
// visible to the subject. Any other case is ErrSiteNotFound: the reader
// never reveals whether the site exists elsewhere.
func (r *SiteReader) readSite(ctx context.Context, tx Tx, organizationID, siteID string) (contracts.SiteSummary, error) {
	if _, err := uuid.Parse(siteID); err != nil {
		return contracts.SiteSummary{}, site.ErrSiteNotFound
	}
	rows, err := tx.Query(ctx, getSiteQuery)
	if err != nil {
		return contracts.SiteSummary{}, err
	}
	sites, err := scanSites(rows)
	if err != nil {
		return contracts.SiteSummary{}, err
	}
	for _, s := range sites {
		if s.ID == siteID && s.OrganizationID == organizationID {
			return s, nil
		}
	}
	return contracts.SiteSummary{}, site.ErrSiteNotFound
}

func (r *SiteReader) readDevices(ctx context.Context, tx Tx, siteID string) ([]contracts.DeviceSummary, error) {
	if _, err := uuid.Parse(siteID); err != nil {
		return nil, site.ErrSiteNotFound
	}
	rows, err := tx.Query(ctx, listSiteDevicesQuery)
	if err != nil {
		return nil, err
	}
	devices, err := scanDevices(rows)
	if err != nil {
		return nil, err
	}
	filtered := make([]contracts.DeviceSummary, 0)
	for _, d := range devices {
		if d.SiteID == siteID {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

func scanSites(rows Rows) ([]contracts.SiteSummary, error) {
	defer rows.Close()
	sites := make([]contracts.SiteSummary, 0)
	for rows.Next() {
		var summary contracts.SiteSummary
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.Timezone, &summary.OrganizationID); err != nil {
			return nil, err
		}
		sites = append(sites, summary)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}
	return sites, nil
}

func scanDevices(rows Rows) ([]contracts.DeviceSummary, error) {
	defer rows.Close()
	devices := make([]contracts.DeviceSummary, 0)
	for rows.Next() {
		var device contracts.DeviceSummary
		if err := rows.Scan(&device.ID, &device.Name, &device.Kind, &device.Status, &device.SiteID); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}
	return devices, nil
}

func (r *SiteReader) validate(subject, organizationID string) (string, string, error) {
	if r == nil || r.db == nil {
		return "", "", site.ErrReaderUnavailable
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", "", authz.ErrUnauthenticated
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return "", "", site.ErrInvalidScope
	}
	return subject, organizationID, nil
}

func (r *SiteReader) validateSite(subject, organizationID, siteID string) (string, string, string, error) {
	subject, organizationID, err := r.validate(subject, organizationID)
	if err != nil {
		return "", "", "", err
	}
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return "", "", "", site.ErrInvalidSiteID
	}
	return subject, organizationID, siteID, nil
}
