package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"energyhub-universal/backend/internal/authz"
	"energyhub-universal/backend/internal/contracts"
)

const listOrganizationsQuery = `
SELECT o.id, o.name, m.role
FROM organization_memberships AS m
JOIN organizations AS o ON o.id = m.organization_id
WHERE m.subject = $1
ORDER BY o.name, o.id`

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

// Querier executes a parameterized query. Implementations must bind args
// positionally and never interpolate caller input into SQL.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type pgxAdapter struct {
	querier pgxQuerier
}

// NewQueryer adapts a pgx pool or connection to the narrow Querier port.
func NewQueryer(querier pgxQuerier) Querier {
	if querier == nil {
		return nil
	}
	return pgxAdapter{querier: querier}
}

func (a pgxAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := a.querier.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// MembershipReader reads organization membership from PostgreSQL. The subject
// is always bound as a query argument.
type MembershipReader struct {
	db Querier
}

func NewMembershipReader(db Querier) *MembershipReader {
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

	rows, err := r.db.Query(ctx, listOrganizationsQuery, subject)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
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
