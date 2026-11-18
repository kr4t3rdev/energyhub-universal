-- Manual rollback of 000005.
--
-- 000005 only revokes privileges that were granted outside the migrations, so
-- there is no privilege this migration created that could be restored. The
-- state before it is the state of 000004, which is already the intended one:
-- no login role other than the privileged migration credential holds a
-- mutation privilege on the protected tables.
--
-- The rollback is therefore a no-op that only re-asserts the 000004 state.
-- It is not meant to give the revoked privileges back: doing so would reopen
-- the write boundary that the migration closed.

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
       AND EXISTS (
           SELECT 1
             FROM unnest(ARRAY[
                 'public.organizations',
                 'public.organization_memberships',
                 'app.admin_audit_log'
             ]) AS protected(table_name)
             CROSS JOIN unnest(ARRAY['INSERT', 'UPDATE', 'DELETE', 'TRUNCATE']) AS privilege(privilege_name)
            WHERE has_table_privilege(role_name.rolname, protected.table_name, privilege.privilege_name)
       );
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'login roles still have write privileges on the protected tables: %', offending;
    END IF;
END
$$;

COMMIT;
