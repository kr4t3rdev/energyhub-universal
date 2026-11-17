package postgres

import (
	"context"
	"strings"

	"energyhub-universal/backend/internal/authz"
)

const createOrganizationFunctionQuery = `SELECT app.create_organization($1, $2)`

// OrganizationWriter creates organizations through the audited admin function.
// It only accepts the admin subject configured at construction.
type OrganizationWriter struct {
	db           DB
	adminSubject string
}

func NewOrganizationWriter(db DB, adminSubject string) *OrganizationWriter {
	return &OrganizationWriter{
		db:           db,
		adminSubject: strings.TrimSpace(adminSubject),
	}
}

func (w *OrganizationWriter) Create(
	ctx context.Context,
	adminSubject string,
	organizationID string,
	name string,
) error {
	if err := w.authorize(adminSubject); err != nil {
		return err
	}
	organizationID = strings.TrimSpace(organizationID)
	name = strings.TrimSpace(name)
	if organizationID == "" || name == "" {
		return authz.ErrInvalidOrganizationTarget
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

	if err := tx.Exec(ctx, createOrganizationFunctionQuery, organizationID, name); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (w *OrganizationWriter) authorize(adminSubject string) error {
	if w == nil || w.db == nil {
		return authz.ErrOrganizationWriterUnavailable
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
