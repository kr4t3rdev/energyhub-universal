-- Manual rollback of 000008_tenant_isolation_roles.
--
-- The rollback does not remove the tenant isolation roles. The whole point of
-- 000008 is that reads of tenant data happen through a dedicated database role
-- per organization (SET ROLE), not through the forgeable app.current_subject
-- GUC. Restoring the previous state would re-allow any statement that runs
-- with the API credential to read every tenant, and a rollback during an
-- incident is exactly the moment where an unnoticed weaker boundary would do
-- the most damage.
--
-- The migrations of this repository are forward-only. Going back to a weaker
-- state is not supported: the way back is to restore a backup, which is an
-- explicit operation of the operator and not a migration.
--
-- This file verifies that the tenant isolation boundary is in place and fails
-- closed when it is not. It changes nothing.

BEGIN;

-- The helper function must exist and belong to the owner role.
IF to_regprocedure('app.tenant_role_name(text)') IS NULL THEN
    RAISE EXCEPTION 'the tenant role function is missing';
END IF;

SELECT string_agg(function.oid::regprocedure::text, ', ' ORDER BY function.oid::regprocedure::text)
  INTO offending
  FROM pg_proc AS function
  JOIN pg_namespace AS schema ON schema.oid = function.pronamespace
  JOIN pg_roles AS owner ON owner.oid = function.proowner
 WHERE schema.nspname = 'app'
   AND function.proname = 'tenant_role_name'
   AND owner.rolname <> 'energyhub_owner';
IF offending IS NOT NULL THEN
    RAISE EXCEPTION 'the tenant role function is not owned by energyhub_owner: %', offending;
END IF;

-- The API runtime credential must be able to use the tenant role name helper.
IF NOT has_function_privilege('energyhub_runtime', 'app.tenant_role_name(text)', 'EXECUTE') THEN
    RAISE EXCEPTION 'the API runtime credential cannot use the tenant role name function';
END IF;

-- No login role other than energyhub_owner may write the tenant tables.
SELECT string_agg(role_name.rolname, ', ' ORDER BY role_name.rolname)
  INTO offending
  FROM pg_roles AS role_name
 WHERE role_name.rolcanlogin
   AND NOT role_name.rolsuper
   AND NOT pg_has_role(role_name.rolname, 'energyhub_owner', 'member')
   AND (
           has_table_privilege(role_name.rolname, 'public.organizations', 'INSERT')
        OR has_table_privilege(role_name.rolname, 'public.organizations', 'UPDATE')
        OR has_table_privilege(role_name.rolname, 'public.organizations', 'DELETE')
        OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'INSERT')
        OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'UPDATE')
        OR has_table_privilege(role_name.rolname, 'public.organization_memberships', 'DELETE')
       );
IF offending IS NOT NULL THEN
    RAISE EXCEPTION 'login roles still have write privileges on tenant tables: %', offending;
END IF;

COMMIT;
