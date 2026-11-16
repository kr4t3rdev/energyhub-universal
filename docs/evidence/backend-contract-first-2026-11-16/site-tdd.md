# Evidencia TDD — frontera tenant-aware

## RED

`go test ./internal/site` no compiló antes de implementar la frontera:

```text
undefined: NewService
undefined: Scope
undefined: ErrInvalidScope
```

La prueba estaba escrita contra el comportamiento deseado y la falla correspondía a la implementación ausente.

## GREEN

Después de añadir `backend/internal/site/service.go`:

- `go test ./internal/site`: PASS.
- Cobertura del paquete `site`: 84.6%.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.

La frontera rechaza scopes incompletos antes de llamar al reader, exige un `organizationID` para cada operación y propaga errores del reader. No autentica tokens ni consulta datos.
