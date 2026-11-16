# Evidencia TDD — pertenencia por organización

## RED

`go test ./internal/authz` no compiló antes de implementar el paquete:

```text
undefined: Principal
undefined: ScopeForOrganization
undefined: ErrOrganizationForbidden
undefined: ErrUnauthenticated
undefined: ErrInvalidOrganization
```

## GREEN

Después de añadir `backend/internal/authz/scope.go`:

- `go test ./internal/authz`: PASS.
- Cobertura del paquete: 100%.
- La membresía conocida produce un `site.Scope`.
- Una organización desconocida no produce scope.
- Subject u organización ausentes se rechazan antes de llegar al servicio.

La función recibe un principal que debe provenir de un verificador futuro; no parsea ni valida tokens.
