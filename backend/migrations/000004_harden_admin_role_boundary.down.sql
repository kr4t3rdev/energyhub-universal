-- Manual rollback of 000004.
--
-- The rollback does not restore the weaker boundary of 000003. The whole point
-- of 000004 is that the administrative write path is a database role
-- boundary, that the administrative credential cannot read the tenant tables
-- and that every administrative write is audited. Restoring the previous
-- state would re-open all of that, and an incident rollback is exactly the
-- moment where an unnoticed weaker boundary would do the most damage.
--
-- The migrations of this repository are forward-only. Going back to a weaker
-- state is not supported: the way back is to restore a backup, which is an
-- explicit operation of the operator and not a migration.
--
-- This file verifies that the hardened state is in place and fails closed
-- when it is not. It changes nothing, and it never removes the audit trail.

BEGIN;

DO $$
DECLARE
    offending text;
BEGIN
    -- The administrative functions must exist and belong to the owner role.
    IF to_regprocedure('app.create_organization(text,text)') IS NULL
       OR to_regprocedure('app.grant_membership(text,text,text)') IS NULL
       OR to_regprocedure('app.revoke_membership(text,text)') IS NULL THEN
        RAISE EXCEPTION 'the administrative functions are missing';
    END IF;

    SELECT string_agg(function.oid::regprocedure::text, ', ' ORDER BY function.oid::regprocedure::text)
      INTO offending
      FROM pg_proc AS function
      JOIN pg_namespace AS schema ON schema.oid = function.pronamespace
      JOIN pg_roles AS owner ON owner.oid = function.proowner
     WHERE schema.nspname = 'app'
       AND function.proname IN ('create_organization', 'grant_membership', 'revoke_membership')
       AND owner.rolname <> 'energyhub_owner';
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'the administrative functions are not owned by energyhub_owner: %', offending;
    END IF;

    -- The independent capability check must still be there.
    IF NOT EXISTS (
        SELECT 1
          FROM pg_proc AS function
          JOIN pg_namespace AS schema ON schema.oid = function.pronamespace
         WHERE schema.nspname = 'app'
           AND function.proname = 'create_organization'
           AND function.prosrc LIKE '%energyhub_admin_capability%'
    ) THEN
        RAISE EXCEPTION 'the administrative functions do not check the capability role';
    END IF;

    -- The administrative credential must not read the tenant tables.
    IF has_table_privilege('energyhub_membership_admin', 'public.organizations', 'SELECT')
       OR has_table_privilege('energyhub_membership_admin', 'public.organization_memberships', 'SELECT') THEN
        RAISE EXCEPTION 'the administrative group can read the tenant tables';
    END IF;

    -- The audit trail must be there.
    IF to_regclass('app.admin_audit_log') IS NULL THEN
        RAISE EXCEPTION 'the audit trail is missing';
    END IF;

    -- The forgeable write policies must not be back.
    IF EXISTS (
        SELECT 1
          FROM pg_policies
         WHERE schemaname = 'public'
           AND policyname IN ('organizations_write_admin', 'memberships_write_admin')
           AND qual LIKE '%app.is_system_admin%'
    ) THEN
        RAISE EXCEPTION 'the forgeable write policies are back';
    END IF;
END
$$;

COMMIT;
