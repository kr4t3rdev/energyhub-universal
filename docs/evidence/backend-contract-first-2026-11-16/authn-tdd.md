# Evidencia TDD — frontera de autenticación

## RED

`go test ./internal/authn` no compiló antes de implementar `BearerToken`:

```text
undefined: BearerToken
undefined: ErrMissingBearerToken
undefined: ErrMalformedBearerToken
```

## GREEN

Después de añadir `backend/internal/authn/bearer.go`, `auth0_config.go` y `auth0.go`:

- `go test ./internal/authn`: PASS.
- Cobertura del paquete: 95.7%.
- Se acepta el esquema Bearer sin distinguir mayúsculas/minúsculas.
- Se rechazan headers ausentes, múltiples, con esquema incorrecto o token malformado.
- `Authenticate` solo llama al `Verifier` después de extraer un Bearer válido y propaga sus errores.
- El middleware solo entrega el principal al handler siguiente cuando el verifier responde sin error; no filtra detalles del verifier.
- `Auth0Verifier` usa discovery OIDC y JWKS para validar firma, issuer, audience y expiración.
- El principal producido contiene únicamente `sub`; la membresía de organizaciones no se confía al token.
