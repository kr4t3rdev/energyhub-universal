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
	// write the protected tables or reach the administrative schema. The
	// runtime credential must be read only.
	ErrRuntimeRoleCanWrite = errors.New("database role can write the protected tables or reach the administrative schema")
	// ErrRuntimeRoleUnknown means the connected role could not be inspected.
	ErrRuntimeRoleUnknown = errors.New("database role could not be inspected")
)

// protectedTables lists the tenant tables the API credential must never be
// able to mutate.
const protectedTables = `ARRAY[
    'public.organizations',
    'public.organization_memberships'
]`

// mutationPrivileges lists every privilege that changes a table. TRUNCATE is
// included because row level security does not apply to it: a role that keeps
// only TRUNCATE can still erase the tenant tables.
const mutationPrivileges = `ARRAY['INSERT', 'UPDATE', 'DELETE', 'TRUNCATE']`

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
//
// The check has two parts: the role must not hold a mutation privilege on the
// tenant tables, and it must not reach the app schema, which holds the
// administrative functions and the audit trail. The audit trail cannot be
// inspected directly because the role is not allowed to resolve its name, so
// the schema access is the control that keeps it out of reach.
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
		`SELECT has_schema_privilege(current_user, 'app', 'USAGE')
            OR has_schema_privilege(current_user, 'app', 'CREATE')
            OR EXISTS (
                 SELECT 1
                   FROM unnest(`+protectedTables+`) AS protected(table_name)
                   CROSS JOIN unnest(`+mutationPrivileges+`) AS privilege(privilege_name)
                  WHERE to_regclass(protected.table_name) IS NOT NULL
                    AND has_table_privilege(current_user, protected.table_name, privilege.privilege_name)
             )`).
		Scan(&canWrite)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRuntimeRoleUnknown, err)
	}
	if canWrite {
		return ErrRuntimeRoleCanWrite
	}
	return nil
}
