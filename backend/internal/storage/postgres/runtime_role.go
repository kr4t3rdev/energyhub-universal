package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrRuntimeRoleIsSuperuser means the API connected with a superuser
	// credential. A superuser bypasses row level security, so the API would
	// read every tenant.
	ErrRuntimeRoleIsSuperuser = errors.New("database role is a superuser")
	// ErrRuntimeRoleBypassesRLS means the API connected with a role that has
	// the BYPASSRLS attribute, which also ignores row level security.
	ErrRuntimeRoleBypassesRLS = errors.New("database role bypasses row level security")
	// ErrRuntimeRoleCanWrite means the API connected with a role that can
	// write the membership tables. The runtime credential must be read only.
	ErrRuntimeRoleCanWrite = errors.New("database role can write the membership tables")
	// ErrRuntimeRoleUnknown means the connected role could not be inspected.
	ErrRuntimeRoleUnknown = errors.New("database role could not be inspected")
)

// RowScanner is the minimal surface of a database row.
type RowScanner interface {
	Scan(dest ...any) error
}

// RoleInspector runs the queries that describe the connected database role.
type RoleInspector interface {
	QueryRow(ctx context.Context, sql string, args ...any) RowScanner
}

// RuntimeRoleVerifier checks that the API connected with a role that cannot
// write and cannot bypass row level security.
type RuntimeRoleVerifier struct {
	inspector RoleInspector
}

// pgxRoleInspector adapts a pgx pool to the RoleInspector port.
type pgxRoleInspector struct {
	pool *pgxpool.Pool
}

func (i pgxRoleInspector) QueryRow(ctx context.Context, sql string, args ...any) RowScanner {
	return i.pool.QueryRow(ctx, sql, args...)
}

// NewPgxRoleInspector adapts a pgx pool to the RoleInspector port.
func NewPgxRoleInspector(pool *pgxpool.Pool) RoleInspector {
	if pool == nil {
		return nil
	}
	return pgxRoleInspector{pool: pool}
}

// NewRuntimeRoleVerifier returns a verifier for the connected database role.
func NewRuntimeRoleVerifier(inspector RoleInspector) *RuntimeRoleVerifier {
	return &RuntimeRoleVerifier{inspector: inspector}
}

// VerifyRuntimeRole fails closed when the connected role is wider than the
// read only credential the API is supposed to use.
func (v *RuntimeRoleVerifier) VerifyRuntimeRole(ctx context.Context) error {
	if v == nil || v.inspector == nil {
		return ErrRuntimeRoleUnknown
	}

	var isSuperuser bool
	var bypassesRLS bool
	err := v.inspector.QueryRow(ctx, "SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user").
		Scan(&isSuperuser, &bypassesRLS)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRuntimeRoleUnknown, err)
	}
	if isSuperuser {
		return ErrRuntimeRoleIsSuperuser
	}
	if bypassesRLS {
		return ErrRuntimeRoleBypassesRLS
	}

	var canWrite bool
	err = v.inspector.QueryRow(ctx,
		"SELECT has_table_privilege(current_user, 'public.organization_memberships', 'INSERT')").
		Scan(&canWrite)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRuntimeRoleUnknown, err)
	}
	if canWrite {
		return ErrRuntimeRoleCanWrite
	}
	return nil
}
