package postgres

import (
	"context"
	"strings"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

const (
	// The writer calls SECURITY DEFINER functions instead of writing tables.
	// The database grants EXECUTE on them only to the membership admin role,
	// so the write boundary is the credential, not a session setting.
	grantMembershipFunctionQuery  = `SELECT app.grant_membership($1, $2, $3)`
	revokeMembershipFunctionQuery = `SELECT app.revoke_membership($1, $2)`
)

// MembershipWriter performs administrative writes. It only accepts the admin
// subject configured at construction and calls the audited admin functions with
// the administrative database credential. authz.AdminMembershipService remains
// the primary gate.
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
	return w.inAdminTx(ctx, func(tx Tx) error {
		return tx.Exec(ctx, grantMembershipFunctionQuery, organizationID, subject, string(role))
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
	return w.inAdminTx(ctx, func(tx Tx) error {
		return tx.Exec(ctx, revokeMembershipFunctionQuery, organizationID, subject)
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

func (w *MembershipWriter) inAdminTx(ctx context.Context, write func(Tx) error) error {
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
