-- Close the audit read of the administrative credential and normalize the
-- privileges of the public schema.
--
-- 000004 granted SELECT on app.admin_audit_log to energyhub_membership_admin
-- so the administrative credential could read the whole audit trail. That
-- credential only has to execute the administrative functions: reading every
-- administrative record of every tenant is not part of its job, and a
-- compromised credential would learn the actors, the organizations, the
-- subjects and the roles of the whole system.
--
-- 000004 and 000005 also revoked CREATE on schema public from the named
-- application roles, but revoking it from them does not override a grant to
-- PUBLIC. A cluster restored from an older dump, or initialized with non
-- default schema privileges, can keep that grant, and an application role
-- could then create shadow objects next to the protected ones.
--
-- It is a new migration on purpose: 000004 and 000005 are already applied in
-- some environments and a published migration is never edited.

BEGIN;

-- The audit trail is read by the privileged deployment role, not by the
-- administrative credential.
REVOKE ALL ON app.admin_audit_log FROM energyhub_membership_admin;
REVOKE ALL ON app.admin_audit_log FROM energyhub_admin;

-- No role may create objects in public, and the administrative schema is not
-- reachable by PUBLIC.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA app FROM PUBLIC;

-- The administrative write path has to keep working: the group still
-- executes the three functions and still uses the schema.
DO $$
BEGIN
    IF NOT has_function_privilege('energyhub_membership_admin', 'app.create_organization(text,text)', 'EXECUTE')
       OR NOT has_function_privilege('energyhub_membership_admin', 'app.grant_membership(text,text,text)', 'EXECUTE')
       OR NOT has_function_privilege('energyhub_membership_admin', 'app.revoke_membership(text,text)', 'EXECUTE') THEN
        RAISE EXCEPTION 'the administrative group cannot execute the administrative functions';
    END IF;
    IF NOT has_schema_privilege('energyhub_membership_admin', 'app', 'USAGE') THEN
        RAISE EXCEPTION 'the administrative group cannot use the administrative schema';
    END IF;
END
$$;

-- Fail closed if a login role can still read the audit trail or create
-- objects in the public schema. The roles that inherit energyhub_owner are
-- excluded on purpose: that is the migration credential, which is privileged
-- by design and must never be the credential of the API. The owner of the
-- database is excluded for the same reason: it holds CREATE on the public
-- schema by definition.
DO $$
DECLARE
    offending text;
BEGIN
    SELECT string_agg(DISTINCT role_name.rolname, ', ' ORDER BY role_name.rolname)
      INTO offending
      FROM pg_roles AS role_name
     WHERE role_name.rolcanlogin
       AND NOT role_name.rolsuper
       AND NOT pg_has_role(role_name.rolname, 'energyhub_owner', 'member')
       AND NOT pg_has_role(role_name.rolname, 'pg_database_owner', 'member')
       AND (has_table_privilege(role_name.rolname, 'app.admin_audit_log', 'SELECT')
            OR has_schema_privilege(role_name.rolname, 'public', 'CREATE'));
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'login roles still hold privileges that the administrative boundary closes: %', offending;
    END IF;
END
$$;

COMMIT;
