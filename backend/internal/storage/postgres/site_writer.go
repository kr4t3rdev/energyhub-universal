package postgres

import (
	"context"
	"strings"

	"energyhub-universal/backend/internal/authz"
)

const (
	// The writer calls SECURITY DEFINER functions instead of writing tables.
	// The database grants EXECUTE on them only to the membership admin role,
	// so the write boundary is the credential, not a session setting.
	createSiteFunctionQuery       = `SELECT app.create_site($1, $2, $3, $4)`
	createSiteDeviceFunctionQuery = `SELECT app.create_site_device($1, $2, $3, $4, $5)`
)

// SiteWriter creates sites and devices through the audited admin functions.
// It only accepts the admin subject configured at construction.
type SiteWriter struct {
	db           DB
	adminSubject string
}

func NewSiteWriter(db DB, adminSubject string) *SiteWriter {
	return &SiteWriter{
		db:           db,
		adminSubject: strings.TrimSpace(adminSubject),
	}
}

func (w *SiteWriter) CreateSite(
	ctx context.Context,
	adminSubject string,
	organizationID string,
	siteID string,
	name string,
	timezone string,
) error {
	if err := w.authorize(adminSubject); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	siteID = strings.TrimSpace(siteID)
	name = strings.TrimSpace(name)
	timezone = strings.TrimSpace(timezone)
	if organizationID == "" || siteID == "" || name == "" || timezone == "" {
		return authz.ErrInvalidSiteTarget
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// app.create_site takes the created site id first, then its parent.
	if err := tx.Exec(ctx, createSiteFunctionQuery, siteID, organizationID, name, timezone); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (w *SiteWriter) CreateDevice(
	ctx context.Context,
	adminSubject string,
	organizationID string,
	siteID string,
	deviceID string,
	name string,
	kind string,
) error {
	if err := w.authorize(adminSubject); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	siteID = strings.TrimSpace(siteID)
	deviceID = strings.TrimSpace(deviceID)
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	if organizationID == "" || siteID == "" || deviceID == "" || name == "" || kind == "" {
		return authz.ErrInvalidSiteTarget
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// app.create_site_device takes the created device id first, then its
	// site and organization.
	if err := tx.Exec(ctx, createSiteDeviceFunctionQuery, deviceID, siteID, organizationID, name, kind); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (w *SiteWriter) authorize(adminSubject string) error {
	if w == nil || w.db == nil {
		return authz.ErrSiteWriterUnavailable
	}
	if w.adminSubject == "" {
		return authz.ErrAdminNotConfigured
	}
	adminSubject = strings.TrimSpace(adminSubject)
	if adminSubject == "" {
		return authz.ErrUnauthenticated
	}
	if adminSubject != w.adminSubject {
		return authz.ErrAdminForbidden
	}
	return nil
}
