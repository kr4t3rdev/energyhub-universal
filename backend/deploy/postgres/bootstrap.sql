-- EnergyHub Universal PostgreSQL roles.
--
-- Run once per cluster as a superuser, before applying the migrations:
--
--   psql "$DATABASE_ADMIN_URL" -v ON_ERROR_STOP=1 \
--     -f backend/deploy/postgres/bootstrap.sql
--
-- This script creates roles and grants membership only. It never assigns a
-- password: credentials are set by the operator from the secret manager, so
-- no credential can end up in the repository. For a local environment, set
-- them yourself with the password coming from the environment:
--
--   psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_runtime PASSWORD '$RUNTIME_PASSWORD'"
--   psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_admin PASSWORD '$ADMIN_PASSWORD'"
--
-- Roles created:
--   energyhub_runtime          read-only credential used by the API
--   energyhub_membership_admin NOLOGIN group that grants administrative writes
--   energyhub_admin            member of the group, administrative credential
--   energyhub_owner            NOLOGIN BYPASSRLS, owner of the admin functions
--
-- The migration that installs the admin functions must run with a role that
-- can change function ownership to energyhub_owner: a superuser, or a role
-- granted membership in energyhub_owner.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'energyhub_membership_admin') THEN
        CREATE ROLE energyhub_membership_admin NOLOGIN;
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

ALTER ROLE energyhub_owner BYPASSRLS;

GRANT energyhub_membership_admin TO energyhub_admin;
