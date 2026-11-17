# Evidencia — backend contract-first

Fecha: 2026-11-16 (actualizado 2026-11-17)

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
- Runner de migraciones: `go test ./internal/storage/postgres` PASS, cobertura 100%; `cmd/migrate` exige la DSN de migración y no se ejecuta desde la API.
- Emisor OIDC local: `go test ./internal/devoidc ./cmd/devoidc` PASS; emite tokens RS256, rechaza credenciales incorrectas y no sirve hosts que no sean loopback.
- Frontera admin por rol: `TEST_DATABASE_URL=... go test ./internal/storage/postgres -run TestIntegration` PASS (9 subcasos sobre PostgreSQL 17.11 real). La escritura admin ya no depende de `app.is_system_admin`, que era falsificable por el rol de la aplicación.
- Ruta protegida y CLI admin: `go test ./cmd/api ./cmd/membership ./internal/platform/httpapi` PASS; 401 sin token o con token manipulado, 200 con las organizaciones del sujeto y ningún detalle interno en las respuestas de error.
- Flujo de extremo a extremo verificado contra PostgreSQL 17.11 y el emisor local: ver `local-stack-e2e.md`.
- El verifier Auth0 valida firma y claims estándar mediante discovery/JWKS y ya está conectado al router protegido; las organizaciones y roles se resuelven en PostgreSQL, nunca desde el token.
