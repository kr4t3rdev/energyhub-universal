-- Manual rollback of 000006.
--
-- 000006 closes the audit read of the administrative credential and removes
-- the CREATE privilege on schema public from PUBLIC. Restoring those
-- privileges would give the administrative credential the whole audit trail
-- of the system and would let any application role create shadow objects
-- next to the protected ones, so the rollback does not restore them.
--
-- The migrations of this repository are forward-only. Going back to a weaker
-- state is not supported: the way back is to restore a backup, which is an
-- explicit operation of the operator and not a migration.
--
-- This file verifies that the restrictive state is in place and fails closed
-- when it is not. It changes nothing.

BEGIN;

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
        RAISE EXCEPTION 'login roles hold privileges that the administrative boundary closes: %', offending;
    END IF;

    IF has_schema_privilege('energyhub_membership_admin', 'public', 'CREATE') THEN
        RAISE EXCEPTION 'the administrative group can create objects in public';
    END IF;
END
$$;

COMMIT;
