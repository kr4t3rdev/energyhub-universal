#!/usr/bin/env bash
# Bring up the local EnergyHub Universal stack for development.
#
# It prepares a local PostgreSQL database with the EnergyHub roles, applies the
# migrations, starts the development OIDC issuer and prints the environment the
# API needs. No real credential or tenant is involved: the database password
# defaults to a development value and the identities are synthetic.
#
# Requirements: a local PostgreSQL server, psql and curl on PATH.
#
#   ./scripts/dev-stack.sh              # prepare everything and print the env
#   ./scripts/dev-stack.sh --with-api   # also start the API in the foreground

set -euo pipefail
umask 077

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
# The issuer process reads its own configuration from the environment, so the
# values have to be exported and not only assigned.
export DEV_OIDC_ADDR DEV_OIDC_AUDIENCE

for command in psql curl; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "${command} is required" >&2
    exit 1
  fi
done

# The values below end up in SQL statements and in connection strings, so they
# are restricted to characters that need no escaping.
if [[ ! "${PGDATABASE}" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]]; then
  echo "PGDATABASE must be a plain identifier, got ${PGDATABASE}" >&2
  exit 1
fi
if [[ ! "${PGHOST}" =~ ^[a-zA-Z0-9._:-]+$ ]]; then
  echo "PGHOST must only use letters, digits and . _ : -" >&2
  exit 1
fi
if [[ ! "${PGPORT}" =~ ^[0-9]{1,5}$ ]]; then
  echo "PGPORT must be a number, got ${PGPORT}" >&2
  exit 1
fi
if [[ ! "${PGUSER}" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]]; then
  echo "PGUSER must be a plain identifier, got ${PGUSER}" >&2
  exit 1
fi
for password in "${RUNTIME_PW}" "${ADMIN_PW}"; do
  if [[ ! "${password}" =~ ^[a-zA-Z0-9._~-]+$ ]]; then
    echo "database passwords must only use letters, digits and . _ ~ -" >&2
    exit 1
  fi
done

if ! psql -d postgres -Atc "select 1" >/dev/null 2>&1; then
  echo "cannot reach PostgreSQL at ${PGHOST}:${PGPORT} as ${PGUSER}" >&2
  exit 1
fi

database_exists="$(psql -d postgres -At -v dbname="${PGDATABASE}" <<'SQL'
select 1 from pg_database where datname = :'dbname';
SQL
)"
if [[ "${database_exists}" != "1" ]]; then
  echo "==> creating database ${PGDATABASE}"
  psql -d postgres -v ON_ERROR_STOP=1 -q -v dbname="${PGDATABASE}" <<'SQL'
CREATE DATABASE :"dbname";
SQL
fi

# The roles are cluster wide, but the app schema belongs to the database, so
# the bootstrap runs against the application database.
echo "==> normalizing roles"
psql -d "${PGDATABASE}" -v ON_ERROR_STOP=1 -q -f "${BACKEND_DIR}/deploy/postgres/bootstrap.sql"

echo "==> setting development role credentials"
# The passwords are read from the environment by psql itself and written to a
# private file, so they neither show up in the process list nor are they
# interpolated into the SQL text.
export RUNTIME_PW ADMIN_PW
CREDENTIALS_FILE="$(mktemp "${TMPDIR:-/tmp}/energyhub-dev-credentials.XXXXXX")"
trap 'rm -f "${CREDENTIALS_FILE}"' EXIT
cat >"${CREDENTIALS_FILE}" <<'SQL'
\getenv runtime_pw RUNTIME_PW
\getenv admin_pw ADMIN_PW
ALTER ROLE energyhub_runtime PASSWORD :'runtime_pw';
ALTER ROLE energyhub_admin PASSWORD :'admin_pw';
SQL
psql -d "${PGDATABASE}" -v ON_ERROR_STOP=1 -q -f "${CREDENTIALS_FILE}"
rm -f "${CREDENTIALS_FILE}"
trap - EXIT

MIGRATION_DSN="postgres://${PGUSER}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"
RUNTIME_DSN="postgres://energyhub_runtime:${RUNTIME_PW}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"
ADMIN_DSN="postgres://energyhub_admin:${ADMIN_PW}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"

echo "==> applying migrations"
(cd "${BACKEND_DIR}" && MIGRATION_DATABASE_URL="${MIGRATION_DSN}" go run ./cmd/migrate)

ISSUER_URL="http://${DEV_OIDC_ADDR}"
issuer_ready() {
  curl --fail --silent --show-error --connect-timeout 1 --max-time 2 \
    --output /dev/null "${ISSUER_URL}/.well-known/openid-configuration"
}
if ! issuer_ready; then
  echo "==> starting development OIDC issuer on ${DEV_OIDC_ADDR}"
  ISSUER_LOG="$(mktemp "${TMPDIR:-/tmp}/energyhub-devoidc.XXXXXX")"
  # The issuer does not need any database credential, so the inherited ones are
  # removed from its environment.
  (cd "${BACKEND_DIR}" && env -u ADMIN_DATABASE_URL -u MIGRATION_DATABASE_URL \
    nohup go run ./cmd/devoidc >"${ISSUER_LOG}" 2>&1 </dev/null &)
  ready=0
  for _ in $(seq 1 30); do
    if issuer_ready; then
      ready=1
      break
    fi
    sleep 0.5
  done
  if [[ "${ready}" != "1" ]]; then
    echo "the development issuer did not start:" >&2
    cat "${ISSUER_LOG}" >&2
    exit 1
  fi
  echo "    issuer log: ${ISSUER_LOG}"
fi

cat <<EOF

Local stack ready.

export AUTH0_ISSUER="${ISSUER_URL}"
export AUTH0_AUDIENCE="${DEV_OIDC_AUDIENCE}"
export SYSTEM_ADMIN_SUBJECT="${SYSTEM_ADMIN_SUBJECT}"
export DATABASE_URL="\${DATABASE_URL:-${RUNTIME_DSN}}"
export ADMIN_DATABASE_URL="\${ADMIN_DATABASE_URL:-${ADMIN_DSN}}"

The connection strings above contain the development passwords and must stay on
this machine. Request a token with the development credentials of the issuer:
the client secret is DEV_OIDC_CLIENT_SECRET and the passwords are
DEV_OIDC_ADMIN_PASSWORD and DEV_OIDC_USER_PASSWORD.

  curl -s -X POST ${ISSUER_URL}/oauth/token -H 'Content-Type: application/json' \\
    -d "{\"client_id\":\"energyhub-dev\",\"client_secret\":\"\${DEV_OIDC_CLIENT_SECRET:-dev-secret}\",\"username\":\"admin\",\"password\":\"\${DEV_OIDC_ADMIN_PASSWORD:-admin-dev}\",\"audience\":\"${DEV_OIDC_AUDIENCE}\"}"

Create an organization and grant a membership (admin only):

  go run ./cmd/membership create-organization org-1 "Demo Org"
  go run ./cmd/membership grant org-1 'dev|user-1' viewer

Start the API:

  go run ./cmd/api

Then call the protected route:

  curl -s http://127.0.0.1:8080/v1/organizations -H "Authorization: Bearer \${TOKEN}"

EOF

if [[ "${1:-}" == "--with-api" ]]; then
  echo "==> starting API"
  cd "${BACKEND_DIR}"
  # The administrative credential belongs to the CLI, never to the API process.
  # The inherited ones are removed as well, so an environment that already has
  # them exported cannot leak them into a long lived process.
  env -u ADMIN_DATABASE_URL -u MIGRATION_DATABASE_URL \
  LISTEN_ADDR="127.0.0.1:8080" \
  AUTH0_ISSUER="${ISSUER_URL}" \
  AUTH0_AUDIENCE="${DEV_OIDC_AUDIENCE}" \
  SYSTEM_ADMIN_SUBJECT="${SYSTEM_ADMIN_SUBJECT}" \
  DATABASE_URL="${RUNTIME_DSN}" \
  go run ./cmd/api
fi
