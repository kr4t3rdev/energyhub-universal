# Evidencia — backend contract-first

Fecha: 2026-11-16

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- OpenAPI YAML parse: PASS (`contracts/openapi/next-phase-proposal.yaml`).
- RED/GREEN del paquete de contratos: `backend/internal/contracts/overview_test.go`.
- Frontera tenant-aware: `go test ./internal/site` PASS, cobertura de `site` 84.6%.
- Resolución de pertenencia: `go test ./internal/authz` PASS, cobertura de `authz` 100%.
- La frontera no registra rutas, autentica tokens ni consulta PostgreSQL.
