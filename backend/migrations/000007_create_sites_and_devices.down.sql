-- Manual rollback of 000007.
--
-- 000007 creates the sites and site_devices tables with tenant-scoped RLS and
-- the audited administrative functions. Dropping the tables would delete the
-- installations and devices of every tenant, and restoring the audit CHECK
-- without the site operations would hide administrative writes that already
-- happened, so the rollback does not drop anything.
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
    IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'sites') THEN
        RAISE EXCEPTION 'public.sites table is missing';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'site_devices') THEN
        RAISE EXCEPTION 'public.site_devices table is missing';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_class
        WHERE relname = 'sites' AND relnamespace = 'public'::regnamespace
          AND relrowsecurity AND relforcerowsecurity
    ) THEN
        RAISE EXCEPTION 'row level security is not forced on public.sites';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_class
        WHERE relname = 'site_devices' AND relnamespace = 'public'::regnamespace
          AND relrowsecurity AND relforcerowsecurity
    ) THEN
        RAISE EXCEPTION 'row level security is not forced on public.site_devices';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'admin_audit_log_operation_check'
          AND pg_get_constraintdef(oid) LIKE '%create_site%'
    ) THEN
        RAISE EXCEPTION 'the audit trail does not record the site operations';
    END IF;

    SELECT string_agg(function.oid::regprocedure::text, ', ' ORDER BY function.oid::regprocedure::text)
      INTO offending
      FROM pg_proc AS function
      JOIN pg_namespace AS schema ON schema.oid = function.pronamespace
      JOIN pg_roles AS owner ON owner.oid = function.proowner
     WHERE schema.nspname = 'app'
       AND function.proname IN ('create_site', 'create_site_device')
       AND owner.rolname <> 'energyhub_owner';
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'the site functions are not owned by energyhub_owner: %', offending;
    END IF;

    SELECT string_agg(DISTINCT role_name.rolname, ', ' ORDER BY role_name.rolname)
      INTO offending
      FROM pg_roles AS role_name
     WHERE role_name.rolcanlogin
       AND NOT role_name.rolsuper
       AND NOT pg_has_role(role_name.rolname, 'energyhub_owner', 'member')
       AND EXISTS (
           SELECT 1
             FROM unnest(ARRAY['public.sites', 'public.site_devices']) AS protected(table_name)
             CROSS JOIN unnest(ARRAY['INSERT', 'UPDATE', 'DELETE', 'TRUNCATE']) AS privilege(privilege_name)
            WHERE has_table_privilege(role_name.rolname, protected.table_name, privilege.privilege_name)
       );
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'login roles still have write privileges on the site tables: %', offending;
    END IF;
END
$$;

COMMIT;
