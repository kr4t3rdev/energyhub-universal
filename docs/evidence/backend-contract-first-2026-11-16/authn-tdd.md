# Evidencia TDD — frontera de autenticación

## RED

`go test ./internal/authn` no compiló antes de implementar `BearerToken`:

```text
undefined: BearerToken
undefined: ErrMissingBearerToken
undefined: ErrMalformedBearerToken
```

## GREEN

Después de añadir `backend/internal/authn/bearer.go`:

- `go test ./internal/authn`: PASS.
- Cobertura del paquete: 100%.
- Se acepta el esquema Bearer sin distinguir mayúsculas/minúsculas.
- Se rechazan headers ausentes, múltiples, con esquema incorrecto o token malformado.
- `Verifier` queda como interfaz sin implementación OIDC/JWT.
