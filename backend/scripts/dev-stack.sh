#!/usr/bin/env bash
# Bring up the local EnergyHub Universal stack for development.
#
# It prepares a local PostgreSQL database with the EnergyHub roles, applies the
# migrations, starts the development OIDC issuer and prints the environment the
# API needs. No real credential or tenant is involved: the database password
# defaults to a development value and the identities are synthetic.
#
# Requirements: a local PostgreSQL server and psql on PATH.
#
#   ./scripts/dev-stack.sh              # prepare everything and print the env
#   ./scripts/dev-stack.sh --with-api   # also start the API in the foreground

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PGHOST="${PGHOST:-127.0.0.1}"
PGPORT="${PGPORT:-5432}"
PGUSER="${PGUSER:-postgres}"
PGDATABASE="${PGDATABASE:-energyhub_dev}"
RUNTIME_PW="${RUNTIME_PW:-dev-runtime}"
ADMIN_PW="${ADMIN_PW:-dev-admin}"
DEV_OIDC_ADDR="${DEV_OIDC_ADDR:-127.0.0.1:8089}"
DEV_OIDC_AUDIENCE="${DEV_OIDC_AUDIENCE:-https://api.energyhub.local}"
SYSTEM_ADMIN_SUBJECT="${SYSTEM_ADMIN_SUBJECT:-dev|admin}"

export PGHOST PGPORT PGUSER

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required" >&2
  exit 1
fi

if ! psql -d postgres -Atc "select 1" >/dev/null 2>&1; then
  echo "cannot reach PostgreSQL at ${PGHOST}:${PGPORT} as ${PGUSER}" >&2
  exit 1
fi

echo "==> creating roles"
psql -d postgres -v ON_ERROR_STOP=1 -q -f "${BACKEND_DIR}/deploy/postgres/bootstrap.sql"

echo "==> setting development role credentials"
psql -d postgres -v ON_ERROR_STOP=1 -q -c "ALTER ROLE energyhub_runtime PASSWORD '${RUNTIME_PW}'"
psql -d postgres -v ON_ERROR_STOP=1 -q -c "ALTER ROLE energyhub_admin PASSWORD '${ADMIN_PW}'"

if ! psql -d postgres -Atc "select 1 from pg_database where datname = '${PGDATABASE}'" | grep -q 1; then
  echo "==> creating database ${PGDATABASE}"
  psql -d postgres -v ON_ERROR_STOP=1 -q -c "CREATE DATABASE ${PGDATABASE}"
fi

MIGRATION_DSN="postgres://${PGUSER}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"
RUNTIME_DSN="postgres://energyhub_runtime:${RUNTIME_PW}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"
ADMIN_DSN="postgres://energyhub_admin:${ADMIN_PW}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"

echo "==> applying migrations"
(cd "${BACKEND_DIR}" && MIGRATION_DATABASE_URL="${MIGRATION_DSN}" go run ./cmd/migrate)

ISSUER_URL="http://${DEV_OIDC_ADDR}"
if ! curl -s -o /dev/null "${ISSUER_URL}/.well-known/openid-configuration"; then
  echo "==> starting development OIDC issuer on ${DEV_OIDC_ADDR}"
  ISUER_LOG="${TMPDIR:-/tmp}/energyhub-devoidc.log"
  (cd "${BACKEND_DIR}" && nohup go run ./cmd/devoidc >"${ISSUER_LOG}" 2>&1 </dev/null &)
  disown || true
  for _ in $(seq 1 30); do
    if curl -s -o /dev/null "${ISSUER_URL}/.well-known/openid-configuration"; then
      break
    fi
    sleep 0.5
  done
  echo "    issuer log: ${ISSUER_LOG}"
fi

cat <<EOF

Local stack ready.

export AUTH0_ISSUER="${ISSUER_URL}"
export AUTH0_AUDIENCE="${DEV_OIDC_AUDIENCE}"
export SYSTEM_ADMIN_SUBJECT="${SYSTEM_ADMIN_SUBJECT}"
export DATABASE_URL="${RUNTIME_DSN}"
export ADMIN_DATABASE_URL="${ADMIN_DSN}"

Request a token as the admin:

  curl -s -X POST ${ISSUER_URL}/oauth/token -H 'Content-Type: application/json' \\
    -d '{"client_id":"energyhub-dev","client_secret":"dev-secret","username":"admin","password":"admin-dev","audience":"${DEV_OIDC_AUDIENCE}"}'

Create an organization and grant a membership (admin only):

  go run ./cmd/membership create-organization org-1 "Demo Org"
  go run ./cmd/membership grant org-1 dev|user-1 viewer

Start the API:

  go run ./cmd/api

Then call the protected route:

  curl -s http://127.0.0.1:8080/v1/organizations -H "Authorization: Bearer \${TOKEN}"

EOF

if [[ "${1:-}" == "--with-api" ]]; then
  echo "==> starting API"
  cd "${BACKEND_DIR}"
  AUTH0_ISSUER="${ISSUER_URL}" \
  AUTH0_AUDIENCE="${DEV_OIDC_AUDIENCE}" \
  SYSTEM_ADMIN_SUBJECT="${SYSTEM_ADMIN_SUBJECT}" \
  DATABASE_URL="${RUNTIME_DSN}" \
  ADMIN_DATABASE_URL="${ADMIN_DSN}" \
  go run ./cmd/api
fi
