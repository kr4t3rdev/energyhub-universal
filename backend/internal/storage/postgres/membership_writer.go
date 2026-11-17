package postgres

import (
	"context"
	"strings"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

const (
	setSystemAdminQuery = `SELECT set_config('app.is_system_admin', 'on', true)`

	upsertMembershipQuery = `
INSERT INTO organization_memberships (organization_id, subject, role)
VALUES ($1, $2, $3)
ON CONFLICT (organization_id, subject)
DO UPDATE SET role = EXCLUDED.role`

	deleteMembershipQuery = `
DELETE FROM organization_memberships
WHERE organization_id = $1 AND subject = $2`
)

// MembershipWriter performs administrative writes. It only accepts the admin
// subject configured at construction and sets both RLS session values inside
// the transaction. authz.AdminMembershipService remains the primary gate.
type MembershipWriter struct {
	db           DB
	adminSubject string
}

func NewMembershipWriter(db DB, adminSubject string) *MembershipWriter {
	return &MembershipWriter{
		db:           db,
		adminSubject: strings.TrimSpace(adminSubject),
	}
}

func (w *MembershipWriter) Grant(
	ctx context.Context,
	adminSubject string,
	organizationID string,
	subject string,
	role contracts.OrganizationRole,
) error {
	adminSubject, organizationID, subject, err := w.authorizeAndNormalize(adminSubject, organizationID, subject)
	if err != nil {
		return err
	}
	if _, err := membershipRole(string(role)); err != nil {
		return err
	}
	return w.inAdminTx(ctx, adminSubject, func(tx Tx) error {
		return tx.Exec(ctx, upsertMembershipQuery, organizationID, subject, string(role))
	})
}

func (w *MembershipWriter) Revoke(
	ctx context.Context,
	adminSubject string,
	organizationID string,
	subject string,
) error {
	adminSubject, organizationID, subject, err := w.authorizeAndNormalize(adminSubject, organizationID, subject)
	if err != nil {
		return err
	}
	return w.inAdminTx(ctx, adminSubject, func(tx Tx) error {
		return tx.Exec(ctx, deleteMembershipQuery, organizationID, subject)
	})
}

func (w *MembershipWriter) authorize(adminSubject string) error {
	if w == nil || w.db == nil {
		return authz.ErrMembershipWriterUnavailable
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

func (w *MembershipWriter) inAdminTx(ctx context.Context, adminSubject string, write func(Tx) error) error {
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

	if err := tx.Exec(ctx, setMembershipSubjectQuery, adminSubject); err != nil {
		return err
	}
	if err := tx.Exec(ctx, setSystemAdminQuery); err != nil {
		return err
	}
	if err := write(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (w *MembershipWriter) authorizeAndNormalize(adminSubject, organizationID, subject string) (string, string, string, error) {
	if err := w.authorize(adminSubject); err != nil {
		return "", "", "", err
	}
	adminSubject = strings.TrimSpace(adminSubject)
	organizationID = strings.TrimSpace(organizationID)
	subject = strings.TrimSpace(subject)
	if organizationID == "" || subject == "" {
		return "", "", "", authz.ErrInvalidMembershipTarget
	}
	return adminSubject, organizationID, subject, nil
}
