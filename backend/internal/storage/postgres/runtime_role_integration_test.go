package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
)

// The startup check has to look at the role PostgreSQL authenticated, not only
// at the role the session currently runs as. A connection can change the
// current role with a startup option, and a session that authenticated as a
// superuser would otherwise pass the check by pretending to be the read only
// runtime role and recover its privileges with a later SET ROLE.

// TestIntegrationRuntimeRoleVerifierRejectsMasqueradingSession proves that a
// session that authenticated as a superuser and changed the current role at
// connection time is rejected.
func TestIntegrationRuntimeRoleVerifierRejectsMasqueradingSession(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	environment := newIntegrationEnvironment(t, superuserURL)

	masqueradingURL := withOption(withDatabase(superuserURL, environment.databaseName), "role="+integrationRuntimeRole)
	pool := connect(t, masqueradingURL)
	defer pool.Close()

	var sessionUser, currentUser string
	if err := pool.QueryRow(ctx, "SELECT session_user, current_user").Scan(&sessionUser, &currentUser); err != nil {
		t.Fatalf("read the session identity: %v", err)
	}
	if sessionUser == currentUser {
		t.Fatalf("session_user = %q and current_user = %q, want a changed role", sessionUser, currentUser)
	}

	verifier := NewRuntimeRoleVerifier(NewPgxRoleInspector(pool))
	err := verifier.VerifyRuntimeRole(ctx)
	if err == nil {
		t.Fatal("err = nil, want the masquerading session to be rejected")
	}
	if !errors.Is(err, ErrRuntimeRoleIsSuperuser) &&
		!errors.Is(err, ErrRuntimeRoleSessionChanged) &&
		!errors.Is(err, ErrRuntimeRoleReachesPrivilegedRole) {
		t.Fatalf("err = %v, want a privileged session error", err)
	}

	// The session really is still privileged: it only has to take its own
	// role back to write, so the startup check is the only thing that stops
	// it from changing every tenant.
	if _, err := pool.Exec(ctx, "SET ROLE "+sessionUser); err != nil {
		t.Fatalf("set the session role back: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO public.organizations (id, name) VALUES ('org-masquerade', 'Masquerade')"); err != nil {
		t.Fatalf("the masquerading session cannot write: %v", err)
	}
}

// TestIntegrationRuntimeRoleVerifierRejectsSetOnlyOwnerMembership proves that a
// runtime role that may assume the owner role without inheriting its
// privileges is rejected. INHERIT FALSE keeps the write privileges out of
// reach, but SET TRUE still allows taking the owner role and writing
// everything afterwards.
func TestIntegrationRuntimeRoleVerifierRejectsSetOnlyOwnerMembership(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	environment := newIntegrationEnvironment(t, superuserURL)

	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	var serverVersion int
	if err := superuserPool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&serverVersion); err != nil {
		t.Fatalf("read the server version: %v", err)
	}
	if serverVersion < 160000 {
		t.Skip("the server does not support separate INHERIT and SET options")
	}

	// The role name is the one the environment created for the runtime
	// credential.
	if _, err := superuserPool.Exec(ctx,
		"GRANT "+integrationOwnerRole+" TO "+environment.runtimeRole+" WITH INHERIT FALSE, SET TRUE"); err != nil {
		t.Fatalf("grant the owner role without inheritance: %v", err)
	}

	runtimePool := connect(t, environment.runtimeURL)
	defer runtimePool.Close()

	// The membership does not leak the write privileges on its own, so the
	// rejection has to come from the reachability check.
	var canInsert bool
	if err := runtimePool.QueryRow(ctx,
		"SELECT has_table_privilege(current_user, 'public.organizations', 'INSERT')").Scan(&canInsert); err != nil {
		t.Fatalf("read the insert privilege: %v", err)
	}
	if canInsert {
		t.Fatal("the role inherited the write privileges of the owner role")
	}

	verifier := NewRuntimeRoleVerifier(NewPgxRoleInspector(runtimePool))
	err := verifier.VerifyRuntimeRole(ctx)
	if !errors.Is(err, ErrRuntimeRoleReachesPrivilegedRole) {
		t.Fatalf("err = %v, want ErrRuntimeRoleReachesPrivilegedRole", err)
	}

	// The membership is real: the role can still take the owner role.
	if _, err := runtimePool.Exec(ctx, "SET ROLE "+integrationOwnerRole); err != nil {
		t.Fatalf("set the owner role: %v", err)
	}
	var isSuperuser bool
	if err := runtimePool.QueryRow(ctx, "SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(&isSuperuser); err != nil {
		t.Fatalf("read the role attribute: %v", err)
	}
	if !isSuperuser {
		t.Fatal("the owner role does not bypass row level security")
	}
}

func TestIntegrationRuntimeRoleVerifierAcceptsTenantSchemaUsage(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)
	ctx := context.Background()
	environment := newIntegrationEnvironment(t, superuserURL)
	createRuntimeRoleTestOrganization(t, ctx, superuserURL, environment, fmt.Sprintf("runtime-role-test-%d", os.Getpid()))

	runtimePool := connect(t, environment.runtimeURL)
	defer runtimePool.Close()

	var hasAppUsage bool
	if err := runtimePool.QueryRow(ctx, "SELECT has_schema_privilege(session_user, 'app', 'USAGE')").Scan(&hasAppUsage); err != nil {
		t.Fatalf("read the inherited tenant schema privilege: %v", err)
	}
	if !hasAppUsage {
		t.Fatal("the runtime role does not inherit the tenant role's app schema usage")
	}

	if err := NewRuntimeRoleVerifier(NewPgxRoleInspector(runtimePool)).VerifyRuntimeRole(ctx); err != nil {
		t.Fatalf("verify the runtime role after tenant-role creation: %v", err)
	}
}

func TestIntegrationRuntimeRoleVerifierRejectsAuditReadAccess(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)
	ctx := context.Background()
	environment := newIntegrationEnvironment(t, superuserURL)
	organizationID := fmt.Sprintf("runtime-role-audit-%d", os.Getpid())
	tenantRole := createRuntimeRoleTestOrganization(t, ctx, superuserURL, environment, organizationID)

	databasePool := connect(t, withDatabase(superuserURL, environment.databaseName))
	defer databasePool.Close()
	var serverVersion int
	if err := databasePool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&serverVersion); err != nil {
		t.Fatalf("read the server version: %v", err)
	}
	if serverVersion < 160000 {
		t.Skip("the server does not support separate INHERIT and SET options")
	}
	if _, err := databasePool.Exec(ctx,
		"GRANT "+tenantRole+" TO "+integrationRuntimeRole+" WITH INHERIT FALSE, SET TRUE"); err != nil {
		t.Fatalf("grant a set-only tenant role: %v", err)
	}
	if _, err := databasePool.Exec(ctx, "GRANT SELECT (actor) ON app.admin_audit_log TO "+tenantRole); err != nil {
		t.Fatalf("grant audit actor-column read access: %v", err)
	}

	runtimePool := connect(t, environment.runtimeURL)
	defer runtimePool.Close()
	err := NewRuntimeRoleVerifier(NewPgxRoleInspector(runtimePool)).VerifyRuntimeRole(ctx)
	if !errors.Is(err, ErrRuntimeRoleCanWrite) {
		t.Fatalf("err = %v, want ErrRuntimeRoleCanWrite", err)
	}
}

func TestIntegrationRuntimeRoleVerifierRejectsAuditSequenceAccess(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)
	ctx := context.Background()
	environment := newIntegrationEnvironment(t, superuserURL)
	tenantRole := createRuntimeRoleTestOrganization(
		t, ctx, superuserURL, environment, fmt.Sprintf("runtime-role-sequence-%d", os.Getpid()),
	)

	databasePool := connect(t, withDatabase(superuserURL, environment.databaseName))
	defer databasePool.Close()
	var sequenceName string
	if err := databasePool.QueryRow(ctx,
		"SELECT pg_get_serial_sequence('app.admin_audit_log', 'id')").Scan(&sequenceName); err != nil {
		t.Fatalf("read the audit sequence name: %v", err)
	}
	if sequenceName == "" {
		t.Fatal("the audit log does not have its expected identity sequence")
	}
	if _, err := databasePool.Exec(ctx, "GRANT USAGE ON SEQUENCE "+sequenceName+" TO "+tenantRole); err != nil {
		t.Fatalf("grant audit sequence access: %v", err)
	}

	runtimePool := connect(t, environment.runtimeURL)
	defer runtimePool.Close()
	err := NewRuntimeRoleVerifier(NewPgxRoleInspector(runtimePool)).VerifyRuntimeRole(ctx)
	if !errors.Is(err, ErrRuntimeRoleCanWrite) {
		t.Fatalf("err = %v, want ErrRuntimeRoleCanWrite", err)
	}
}

func createRuntimeRoleTestOrganization(
	t *testing.T,
	ctx context.Context,
	superuserURL string,
	environment integrationEnvironment,
	organizationID string,
) string {
	t.Helper()

	adminPool := connect(t, environment.adminURL)
	if _, err := adminPool.Exec(ctx, "SELECT app.create_organization($1, $2)", organizationID, "Runtime Role Test"); err != nil {
		adminPool.Close()
		t.Fatalf("create a test organization through the admin function: %v", err)
	}
	adminPool.Close()

	databasePool := connect(t, withDatabase(superuserURL, environment.databaseName))
	var tenantRole string
	if err := databasePool.QueryRow(ctx, "SELECT app.tenant_role_name($1)", organizationID).Scan(&tenantRole); err != nil {
		databasePool.Close()
		t.Fatalf("read the generated tenant role name: %v", err)
	}
	databasePool.Close()

	t.Cleanup(func() {
		cleanupDatabasePool := connect(t, withDatabase(superuserURL, environment.databaseName))
		if _, err := cleanupDatabasePool.Exec(context.Background(), "DROP OWNED BY "+tenantRole); err != nil {
			t.Errorf("drop tenant role grants: %v", err)
		}
		cleanupDatabasePool.Close()

		cleanupClusterPool := connect(t, superuserURL)
		defer cleanupClusterPool.Close()
		if _, err := cleanupClusterPool.Exec(context.Background(),
			"REVOKE "+tenantRole+" FROM "+integrationRuntimeRole+", "+integrationOwnerRole+" CASCADE"); err != nil {
			t.Errorf("revoke the test tenant role: %v", err)
		}
		if _, err := cleanupClusterPool.Exec(context.Background(), "DROP ROLE IF EXISTS "+tenantRole); err != nil {
			t.Errorf("drop the test tenant role: %v", err)
		}
	})
	return tenantRole
}

// TestIntegrationRuntimeRoleVerifierRejectsSharedCredential proves that the
// credential the API uses is rejected when another login role can assume it.
//
// The check walks the membership graph backwards, so it is the authenticated
// role that must not be held by a second login role. A second login role would
// read every tenant, because the read isolation trusts a session variable that
// the holder of the credential can set.
func TestIntegrationRuntimeRoleVerifierRejectsSharedCredential(t *testing.T) {
	superuserURL := integrationSuperuserURL(t)

	ctx := context.Background()
	environment := newIntegrationEnvironment(t, superuserURL)

	superuserPool := connect(t, superuserURL)
	defer superuserPool.Close()

	// A second login role that can take the credential of the API. The grant
	// is on the role the API authenticates as, which is the ephemeral role the
	// environment created for it.
	sharedRole := createEphemeralRole(t, ctx, superuserURL, "energyhub_it_sharer", environment.runtimeRole)

	runtimePool := connect(t, environment.runtimeURL)
	defer runtimePool.Close()

	verifier := NewRuntimeRoleVerifier(NewPgxRoleInspector(runtimePool))
	err := verifier.VerifyRuntimeRole(ctx)
	if err == nil {
		t.Fatal("err = nil, want the shared credential to be rejected")
	}
	if !errors.Is(err, ErrRuntimeRoleSharedCredential) {
		t.Fatalf("err = %v, want ErrRuntimeRoleSharedCredential", err)
	}

	// The share is real: the login role can take the credential of the API and
	// read the tenant tables with it.
	sharedPool := connect(t, withCredentials(superuserURL, sharedRole, integrationPassword, environment.databaseName))
	defer sharedPool.Close()
	if _, err := sharedPool.Exec(ctx, "SET ROLE "+environment.runtimeRole); err != nil {
		t.Fatalf("take the credential of the API: %v", err)
	}
	var canRead bool
	if err := sharedPool.QueryRow(ctx,
		"SELECT has_table_privilege(current_user, 'public.organizations', 'SELECT')").Scan(&canRead); err != nil {
		t.Fatalf("read the privilege of the shared credential: %v", err)
	}
	if !canRead {
		t.Fatal("the shared role cannot read the tenant tables with the credential of the API")
	}

	// Removing the membership makes the deployment acceptable again, so the
	// check is about the share and not about the role itself.
	if _, err := superuserPool.Exec(ctx, "REVOKE "+environment.runtimeRole+" FROM "+sharedRole); err != nil {
		t.Fatalf("revoke the shared role: %v", err)
	}
	if err := verifier.VerifyRuntimeRole(ctx); err != nil {
		t.Fatalf("verify after removing the share: %v", err)
	}
}

// withOption appends a PostgreSQL connection option to a connection string.
//
// The value is percent encoded: PostgreSQL reads the query string of the
// connection string literally, so a space has to be %20 and not the plus sign
// of the form encoding.
func withOption(databaseURL string, option string) string {
	separator := "?"
	if strings.Contains(databaseURL, "?") {
		separator = "&"
	}
	encoded := strings.ReplaceAll(url.QueryEscape(option), "+", "%20")
	return databaseURL + separator + "options=-c%20" + encoded
}
