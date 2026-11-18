-- Remove the mutation privileges that 000004 could not detect.
--
-- 000004 revoked the privileges it knew about, but its detection list only
-- covered INSERT, UPDATE and DELETE. A login role that kept only TRUNCATE was
-- therefore left with a way to erase the tenant tables, because row level
-- security does not apply to TRUNCATE.
--
-- This migration revokes every mutation privilege from every login role that
-- does not inherit energyhub_owner, and then fails closed if any of them is
-- still able to change a protected table. The audit trail is included: a role
-- that could write it could forge administrative records.
--
-- It is a new migration on purpose: 000004 is already applied in some
-- environments and a published migration is never edited.

BEGIN;

-- Function ownership --------------------------------------------------------------

-- CREATE OR REPLACE keeps the previous owner, but the functions may not exist
-- at all when the schema was restored or a previous migration was rolled back.
-- The owner is re-asserted so the administrative functions never depend on
-- the credential that installed them.
ALTER FUNCTION app.create_organization(text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.grant_membership(text, text, text) OWNER TO energyhub_owner;
ALTER FUNCTION app.revoke_membership(text, text) OWNER TO energyhub_owner;

DO $$
DECLARE
    offending text;
BEGIN
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
END
$$;

-- Leftover privileges -------------------------------------------------------------

-- The revoke is unconditional: no role other than the privileged migration
-- credential has a reason to mutate these tables. The roles that inherit
-- energyhub_owner are excluded because that is the deployment job.
--
-- The revoke also covers the group roles, because a privilege granted to a
-- group is inherited by its members and cannot be taken away by revoking it
-- from the member alone.
DO $$
DECLARE
    role_name text;
BEGIN
    FOR role_name IN
        SELECT rolname
          FROM pg_roles
         WHERE NOT rolsuper
           AND NOT pg_has_role(rolname, 'energyhub_owner', 'member')
    LOOP
        EXECUTE format(
            'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.organizations, public.organization_memberships, app.admin_audit_log FROM %I',
            role_name);
    END LOOP;
END
$$;

-- Fail closed if a login role can still change a protected table.
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
