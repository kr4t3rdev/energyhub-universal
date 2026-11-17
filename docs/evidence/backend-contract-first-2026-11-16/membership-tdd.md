# Evidencia TDD — membresías server-side

## RED

`go test ./internal/authz` no compiló antes de añadir el puerto de membresías:

```text
undefined: NewMembershipAuthorizer
undefined: ErrMembershipReaderUnavailable
```

## GREEN

- `MembershipReader` recibe siempre el `subject` ya verificado.
- `MembershipAuthorizer` carga las organizaciones desde esa fuente y solo después construye el scope.
- Una organización fuera de las membresías devuelve `ErrOrganizationForbidden`.
- Subject ausente y reader ausente se rechazan antes de acceder a datos.
- `go test ./internal/authz`: PASS, cobertura 100%.

No se añadió implementación PostgreSQL ni se aceptan organizaciones o roles desde headers o claims del token.
