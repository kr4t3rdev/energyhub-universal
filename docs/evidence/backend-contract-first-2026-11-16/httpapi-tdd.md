# Evidencia TDD — correlación y errores HTTP

## RED

`go test ./internal/platform/httpapi` no compiló antes de implementar la base HTTP:

```text
undefined: WriteErrorResponse
undefined: RequestIDHeader
undefined: RequestIDFromContext
undefined: RequestIDMiddleware
```

## GREEN

Después de añadir `backend/internal/platform/httpapi/request.go` y `errors.go`:

- `go test ./internal/platform/httpapi`: PASS.
- Cobertura del paquete: 96.6%.
- Cada request de `NewRouter` recibe un `X-Request-ID` generado por el servidor.
- El mismo identificador queda disponible en el contexto downstream.
- Los errores estructurados usan `contracts.ErrorResponse`.
- La falla del generador se prueba sin depender de la aleatoriedad real.

Esta pieza no agrega rutas de negocio ni cambia `/health`: sigue siendo liveness del proceso.
