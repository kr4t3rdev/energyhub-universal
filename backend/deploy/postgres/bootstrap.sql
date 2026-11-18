-- EnergyHub Universal PostgreSQL roles.
--
-- Run once per cluster as a superuser, against the application database,
-- before applying the migrations:
--
--   psql "$DATABASE_ADMIN_URL" -v ON_ERROR_STOP=1 \
--     -f backend/deploy/postgres/bootstrap.sql
--
-- This script creates and normalizes roles and prepares the app schema. It
-- never assigns a password: credentials are set by the operator from the
-- secret manager, so no credential can end up in the repository. For a local
-- environment, set them yourself with the password coming from the
-- environment:
--
--   psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_runtime PASSWORD '$RUNTIME_PASSWORD'"
--   psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_admin PASSWORD '$ADMIN_PASSWORD'"
--
-- Roles:
--   energyhub_runtime          read-only credential used by the API
--   energyhub_membership_admin NOLOGIN group that grants the EXECUTE rights
--   energyhub_admin_capability NOLOGIN group checked inside the admin functions
--   energyhub_admin            member of both groups, administrative credential
--   energyhub_owner            NOLOGIN BYPASSRLS, owner of the protected objects
--
-- The migration that installs the admin functions must run with a role that
-- can change function ownership to energyhub_owner: a superuser, or a role
-- granted membership in energyhub_owner. The app schema is created here with
-- energyhub_owner as owner so that such a migrator can install the functions.
--
-- A role that inherits energyhub_owner holds BYPASSRLS and direct write
-- privileges, so that credential belongs to the deployment job only and must
-- never be the DATABASE_URL of the API.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_membership_admin') THEN
        CREATE ROLE energyhub_membership_admin NOLOGIN;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_admin_capability') THEN
        CREATE ROLE energyhub_admin_capability NOLOGIN;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_owner') THEN
        CREATE ROLE energyhub_owner NOLOGIN BYPASSRLS;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_runtime') THEN
        CREATE ROLE energyhub_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_admin') THEN
        CREATE ROLE energyhub_admin LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;
END
$$;

-- Normalization ------------------------------------------------------------------
--
-- A role that already existed keeps its attributes unless they are set here.
-- The attributes are written on every run so a role that was created wider by
-- mistake loses the extra privileges.

ALTER ROLE energyhub_membership_admin NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE energyhub_admin_capability NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE energyhub_owner NOLOGIN NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE energyhub_runtime LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE energyhub_admin LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;

-- The application credentials must not reach the administrative groups, and
-- the administrative credential must not reach the owner role.
REVOKE energyhub_membership_admin FROM energyhub_runtime, energyhub_owner, energyhub_admin;
REVOKE energyhub_admin_capability FROM energyhub_runtime, energyhub_owner, energyhub_admin, energyhub_membership_admin;
REVOKE energyhub_owner FROM energyhub_runtime, energyhub_admin, energyhub_membership_admin;

GRANT energyhub_membership_admin TO energyhub_admin;
GRANT energyhub_admin_capability TO energyhub_admin;

-- The app schema belongs to energyhub_owner, so the admin functions and the
-- audit trail are owned by a role that cannot log in. It also needs CREATE on
-- public to own the tenant tables.
CREATE SCHEMA IF NOT EXISTS app AUTHORIZATION energyhub_owner;
REVOKE CREATE ON SCHEMA app FROM PUBLIC;
GRANT CREATE ON SCHEMA public TO energyhub_owner;

-- Assertions ---------------------------------------------------------------------

DO $$
DECLARE
    offending text;
BEGIN
    SELECT string_agg(DISTINCT member.rolname, ', ' ORDER BY member.rolname)
      INTO offending
      FROM pg_auth_members AS membership
      JOIN pg_roles AS granted ON granted.oid = membership.roleid
      JOIN pg_roles AS member ON member.oid = membership.member
     WHERE granted.rolname = 'energyhub_membership_admin'
       AND member.rolname <> 'energyhub_admin';
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'unexpected members of energyhub_membership_admin: %', offending;
    END IF;

    SELECT string_agg(DISTINCT member.rolname, ', ' ORDER BY member.rolname)
      INTO offending
      FROM pg_auth_members AS membership
      JOIN pg_roles AS granted ON granted.oid = membership.roleid
      JOIN pg_roles AS member ON member.oid = membership.member
     WHERE granted.rolname = 'energyhub_admin_capability'
       AND member.rolname <> 'energyhub_admin';
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'unexpected members of energyhub_admin_capability: %', offending;
    END IF;

    SELECT string_agg(DISTINCT member.rolname, ', ' ORDER BY member.rolname)
      INTO offending
      FROM pg_auth_members AS membership
      JOIN pg_roles AS granted ON granted.oid = membership.roleid
      JOIN pg_roles AS member ON member.oid = membership.member
     WHERE granted.rolname = 'energyhub_owner'
       AND member.rolname IN ('energyhub_runtime', 'energyhub_admin', 'energyhub_membership_admin');
    IF offending IS NOT NULL THEN
        RAISE EXCEPTION 'application roles must not be members of energyhub_owner: %', offending;
    END IF;
END
$$;
