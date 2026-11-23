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
	// ErrRuntimeRoleSessionChanged means the session does not run as the role
	// PostgreSQL authenticated. A connection can change the current role with
	// SET ROLE or with a startup option, so a check that only looks at the
	// current role can be passed by a privileged session that recovers its
	// privileges afterwards.
	ErrRuntimeRoleSessionChanged = errors.New("database session does not run as the authenticated role")
	// ErrRuntimeRoleReachesPrivilegedRole means the authenticated role can
	// assume a role that owns the objects or holds the administrative
	// capability, even when it does not inherit those privileges yet.
	ErrRuntimeRoleReachesPrivilegedRole = errors.New("database role can assume a privileged database role")
	// ErrRuntimeRoleSharedCredential means another login role can assume the
	// role the API authenticated as. Sharing the credential is the same
	// problem as sharing the administrative credential: a second login role
	// would read every tenant, because the read isolation trusts a session
	// variable that any session can set.
	ErrRuntimeRoleSharedCredential = errors.New("database role is shared with another login role")
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

// privilegedRoles lists the roles the API credential must never be able to
// assume. Reaching one of them is enough to write, to bypass row level
// security or to replace the administrative functions.
const privilegedRoles = `ARRAY[
    'energyhub_owner',
    'energyhub_admin',
    'energyhub_membership_admin',
    'energyhub_admin_capability'
]`

// sessionIdentityQuery reads the identity PostgreSQL authenticated and
// compares it with the role the session currently runs as.
//
// The attributes of the session role are the ones that matter: current_user
// can be changed with SET ROLE or with a startup option such as
// "options=-c role=energyhub_runtime", and a session that authenticated as a
// superuser would otherwise pass the check by pretending to be the read only
// runtime role.
const sessionIdentityQuery = `
SELECT session_user = current_user,
       session_role.rolsuper,
       session_role.rolbypassrls
  FROM pg_roles AS session_role
 WHERE session_role.rolname = session_user`

// roleReachabilityQuery resolves every role the authenticated role can become
// and checks the privileges the API credential must not have.
//
// The reachability is resolved through the whole membership graph, because a
// membership with INHERIT FALSE still allows SET ROLE: such a role has no
// inherited write privilege, but it can take the owner role and write
// everything afterwards.
//
// The graph is also walked in the opposite direction, to find the roles that
// can assume the authenticated one. A credential that another login role can
// take is a shared credential, and a shared read credential reads every
// tenant: the read isolation trusts a session variable that the holder of the
// credential can set.
const roleReachabilityQuery = `
WITH RECURSIVE
reachable(role_oid) AS (
    SELECT role_oid.oid
      FROM pg_roles AS role_oid
     WHERE role_oid.rolname = session_user
    UNION
    SELECT membership.roleid
      FROM pg_auth_members AS membership
      JOIN reachable ON membership.member = reachable.role_oid
),
assumers(role_oid) AS (
    SELECT membership.member
      FROM pg_auth_members AS membership
      JOIN pg_roles AS granted ON granted.oid = membership.roleid
     WHERE granted.rolname = session_user
    UNION
    SELECT membership.member
      FROM pg_auth_members AS membership
      JOIN assumers ON membership.roleid = assumers.role_oid
)
SELECT
    EXISTS (
        SELECT 1
          FROM reachable
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
         WHERE candidate.rolname::text = ANY (` + privilegedRoles + `)
    ),
    -- Tenant roles need USAGE on app for RLS helpers; reject CREATE and
    -- direct access to administrative tables or SECURITY DEFINER functions.
    EXISTS (
        SELECT 1
          FROM reachable
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
         WHERE candidate.rolsuper OR candidate.rolbypassrls
    ),
    EXISTS (
        SELECT 1
          FROM reachable
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
         WHERE to_regnamespace('app') IS NOT NULL
           AND has_schema_privilege(candidate.rolname, 'app', 'CREATE')
    )
    OR EXISTS (
        SELECT 1
          FROM pg_class AS administrative_object
          JOIN pg_namespace AS administrative_schema
            ON administrative_schema.oid = administrative_object.relnamespace
          JOIN reachable ON TRUE
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
          CROSS JOIN unnest(ARRAY['SELECT', 'INSERT', 'UPDATE', 'DELETE', 'TRUNCATE', 'REFERENCES', 'TRIGGER']) AS privilege(privilege_name)
         WHERE administrative_schema.nspname = 'app'
           AND administrative_object.relkind IN ('r', 'p', 'v', 'm')
           AND has_table_privilege(candidate.rolname, administrative_object.oid, privilege.privilege_name)
    )
    OR EXISTS (
        SELECT 1
          FROM pg_class AS administrative_object
          JOIN pg_namespace AS administrative_schema
            ON administrative_schema.oid = administrative_object.relnamespace
          JOIN reachable ON TRUE
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
          CROSS JOIN unnest(ARRAY['SELECT', 'INSERT', 'UPDATE', 'REFERENCES']) AS privilege(privilege_name)
         WHERE administrative_schema.nspname = 'app'
           AND administrative_object.relkind IN ('r', 'p', 'v', 'm')
           AND has_any_column_privilege(candidate.rolname, administrative_object.oid, privilege.privilege_name)
    )
    OR EXISTS (
        SELECT 1
          FROM pg_class AS administrative_sequence
          JOIN pg_namespace AS administrative_schema
            ON administrative_schema.oid = administrative_sequence.relnamespace
          JOIN reachable ON TRUE
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
          CROSS JOIN unnest(ARRAY['USAGE', 'SELECT', 'UPDATE']) AS privilege(privilege_name)
         WHERE administrative_schema.nspname = 'app'
           AND administrative_sequence.relkind = 'S'
           AND has_sequence_privilege(candidate.rolname, administrative_sequence.oid, privilege.privilege_name)
    )
    OR EXISTS (
        SELECT 1
          FROM pg_proc AS administrative_function
          JOIN pg_namespace AS administrative_schema
            ON administrative_schema.oid = administrative_function.pronamespace
          JOIN reachable ON TRUE
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
         WHERE administrative_schema.nspname = 'app'
           AND administrative_function.prosecdef
           AND has_function_privilege(candidate.rolname, administrative_function.oid, 'EXECUTE')
    )
    OR EXISTS (
        SELECT 1
          FROM reachable
          JOIN pg_roles AS candidate ON candidate.oid = reachable.role_oid
          CROSS JOIN unnest(` + protectedTables + `) AS protected(table_name)
          CROSS JOIN unnest(` + mutationPrivileges + `) AS privilege(privilege_name)
         WHERE to_regclass(protected.table_name) IS NOT NULL
           AND has_table_privilege(candidate.rolname, protected.table_name, privilege.privilege_name)
    ),
    EXISTS (
        SELECT 1
          FROM assumers
          JOIN pg_roles AS candidate ON candidate.oid = assumers.role_oid
         WHERE candidate.rolcanlogin
           AND NOT candidate.rolsuper
           AND candidate.rolname <> session_user
    )`

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
// The check has four parts: the authenticated role must not be a superuser
// nor bypass row level security, the session must still run as that role, the
// role must neither hold a mutation privilege on the tenant tables nor be
// able to assume a privileged one, and no other login role may assume it.
// Tenant roles may use the app schema for row level security helpers, so the
// verifier checks all reachable roles against administrative table, column,
// sequence, and SECURITY DEFINER function privileges directly.
func (v *RuntimeRoleVerifier) VerifyRuntimeRole(ctx context.Context) error {
	if v == nil || v.inspector == nil {
		return ErrRuntimeRoleUnknown
	}

	var sessionIsCurrent bool
	var isSuperuser bool
	var bypassesRLS bool
	err := v.inspector.QueryRow(ctx, sessionIdentityQuery).
		Scan(&sessionIsCurrent, &isSuperuser, &bypassesRLS)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRuntimeRoleUnknown, err)
	}
	if isSuperuser {
		return ErrRuntimeRoleIsSuperuser
	}
	if bypassesRLS {
		return ErrRuntimeRoleBypassesRLS
	}
	if !sessionIsCurrent {
		return ErrRuntimeRoleSessionChanged
	}

	var reachesPrivilegedRole bool
	var reachesUnsafeRole bool
	var canWrite bool
	var sharedCredential bool
	err = v.inspector.QueryRow(ctx, roleReachabilityQuery).
		Scan(&reachesPrivilegedRole, &reachesUnsafeRole, &canWrite, &sharedCredential)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRuntimeRoleUnknown, err)
	}
	if reachesPrivilegedRole || reachesUnsafeRole {
		return ErrRuntimeRoleReachesPrivilegedRole
	}
	if canWrite {
		return ErrRuntimeRoleCanWrite
	}
	if sharedCredential {
		return ErrRuntimeRoleSharedCredential
	}
	return nil
}
