package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

const (
	listOrganizationsQuery = `
SELECT o.id, o.name, m.role
FROM organization_memberships AS m
JOIN organizations AS o ON o.id = m.organization_id
WHERE m.subject = $1
ORDER BY o.name, o.id`

	// setMembershipSubjectQuery sets the transaction-local subject that RLS
	// policies use. The value is bound, never interpolated.
	setMembershipSubjectQuery = `SELECT set_config('app.current_subject', $1, true)`
)

var ErrInvalidMembershipRole = errors.New("invalid membership role")

// Rows is the minimal result-set surface used by the reader. Keeping it narrow
// lets tests exercise scanning without reimplementing the whole pgx.Rows
// interface, which gained methods in pgx v5.13.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Tx is the transaction surface used for RLS-scoped reads and administrative
// writes. Exec discards the command tag; Query returns the result set.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type DB interface {
	Begin(ctx context.Context) (Tx, error)
}

type pgxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

var _ pgxBeginner = (*pgxpool.Pool)(nil)

type pgxDB struct {
	beginner pgxBeginner
}

// NewDB adapts a pgx pool or connection to the narrow DB port.
func NewDB(beginner pgxBeginner) DB {
	if beginner == nil {
		return nil
	}
	return pgxDB{beginner: beginner}
}

func (d pgxDB) Begin(ctx context.Context) (Tx, error) {
	tx, err := d.beginner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxTx{tx: tx}, nil
}

type pgxTx struct {
	tx pgx.Tx
}

func (t pgxTx) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := t.tx.Exec(ctx, sql, args...)
	return err
}

func (t pgxTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (t pgxTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t pgxTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// MembershipReader reads organization membership from PostgreSQL. Every read
// runs in a transaction that sets app.current_subject so RLS can enforce the
// same subject even if a future query forgets the WHERE clause.
type MembershipReader struct {
	db DB
}

func NewMembershipReader(db DB) *MembershipReader {
	return &MembershipReader{db: db}
}

func (r *MembershipReader) ListOrganizations(ctx context.Context, subject string) ([]contracts.OrganizationSummary, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, authz.ErrUnauthenticated
	}
	if r == nil || r.db == nil {
		return nil, authz.ErrMembershipReaderUnavailable
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

	if err := tx.Exec(ctx, setMembershipSubjectQuery, subject); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, listOrganizationsQuery, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	organizations := make([]contracts.OrganizationSummary, 0)
	for rows.Next() {
		var organization contracts.OrganizationSummary
		var role string
		if err := rows.Scan(&organization.ID, &organization.Name, &role); err != nil {
			return nil, err
		}
		organization.Role, err = membershipRole(role)
		if err != nil {
			return nil, err
		}
		organizations = append(organizations, organization)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true
	return organizations, nil
}

func membershipRole(value string) (contracts.OrganizationRole, error) {
	switch contracts.OrganizationRole(value) {
	case contracts.RoleOwner, contracts.RoleAdmin, contracts.RoleMember, contracts.RoleViewer:
		return contracts.OrganizationRole(value), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidMembershipRole, value)
	}
}
