# Evidencia TDD — verifier Auth0

## RED

`go test ./internal/authn` no compiló antes de añadir la configuración y el verifier:

```text
undefined: LoadAuth0Config
undefined: Auth0Verifier
undefined: ErrInvalidAuth0Config
```

## GREEN

- `LoadAuth0Config` exige `AUTH0_ISSUER` y `AUTH0_AUDIENCE`.
- El issuer solo acepta HTTPS sin credenciales, query ni fragment.
- `NewAuth0Verifier` usa discovery OIDC y el endpoint JWKS descubierto.
- Una prueba TLS local genera un JWT RS256, publica su JWKS y verifica `iss`, `aud`, firma, `iat`, `exp` y `sub`.
- Los errores del verificador no exponen detalles internos.
- Organizaciones y roles no se extraen del token; quedan para autorización server-side.
