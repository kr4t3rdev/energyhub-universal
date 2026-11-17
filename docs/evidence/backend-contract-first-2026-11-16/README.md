# Evidencia — backend contract-first

Fecha: 2026-11-16

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- OpenAPI YAML parse: PASS (`contracts/openapi/next-phase-proposal.yaml`).
- RED/GREEN del paquete de contratos: `backend/internal/contracts/overview_test.go`.
- Frontera tenant-aware: `go test ./internal/site` PASS, cobertura de `site` 84.6%.
- Resolución de pertenencia: `go test ./internal/authz` PASS, cobertura de `authz` 100%.
- Autenticación Auth0/Bearer: `go test ./internal/authn` PASS, cobertura de `authn` 95.7%.
- Base HTTP: `go test ./internal/platform/httpapi` PASS, cobertura de `httpapi` 96.6%.
- Membresías PostgreSQL: `go test ./internal/storage/postgres` PASS, cobertura 100%, sin base real.
- RLS y admin: `go test ./internal/authz ./internal/storage/postgres` PASS, cobertura 100%; solo `SYSTEM_ADMIN_SUBJECT` puede escribir membresías y la configuración vacía falla cerrada.
- Runner de migraciones: `go test ./internal/storage/postgres` PASS, cobertura 100%; `cmd/migrate` exige `DATABASE_URL` y no se ejecuta desde la API.
- El verifier Auth0 valida firma y claims estándar mediante discovery/JWKS, pero aún no se registra en `NewRouter` ni consulta PostgreSQL.
