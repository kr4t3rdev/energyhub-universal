# Diseño de la siguiente fase backend

Estado: diseño aprobado para contract-first; modelos de respuesta y verifier Auth0 implementados. El verifier aún no se registra en rutas protegidas; no hay endpoints de datos ni acceso a PostgreSQL.

## Límites de la entrega actual

- `GET /health` sigue siendo únicamente una señal de vida del proceso.
- La web consume `FixtureSiteOverviewProvider`; no hace peticiones al backend.
- No se muestra información de usuarios, organizaciones, dispositivos reales ni telemetría.
- El contrato propuesto en `contracts/openapi/next-phase-proposal.yaml` es un artefacto de diseño y no declara disponibilidad de esos endpoints.

## Autenticación propuesta

1. Usar Auth0 como proveedor OIDC compatible con OAuth 2.1.
2. Los clientes obtienen un access token de corta duración y lo envían como `Authorization: Bearer <token>`.
3. Go valida firma mediante JWKS cacheado, `iss`, `aud`, `exp`, `nbf` y `sub`. Un token inválido o ausente produce `401`; no se debe consultar la base de datos antes de esa validación.
4. El backend no aceptará `organization_id`, `tenant_id` ni roles desde cabeceras controladas por el cliente. La pertenencia y el rol se resolverán en el servidor.
5. Refresh tokens, si se usan, permanecen bajo control del proveedor y de los clientes nativos; nunca se almacenan en PostgreSQL como texto legible.
6. Los logs deben incluir un identificador de correlación y el sujeto anonimizado; nunca tokens, cookies ni datos energéticos completos.

## Aislamiento por organización

- Cada recurso de negocio persistido tendrá `organization_id NOT NULL`.
- La autorización se comprobará en cada caso de uso: `subject -> organization membership -> resource`.
- Las consultas siempre filtrarán por `organization_id` y por el identificador del recurso. No habrá consultas por `site_id` global sin el ámbito de organización.
- La capa de repositorio recibirá el contexto autorizado, no un tenant elegido libremente por la UI.
- Se recomienda activar RLS como defensa adicional cuando se introduzca PostgreSQL, con una transacción que establezca el contexto autorizado. RLS no sustituye la autorización en Go.
- Respuestas para una organización inexistente o un recurso ajeno deben evitar filtrar su existencia; según el caso se responderá `403` o `404` de forma consistente.
- Roles iniciales propuestos: `owner`, `admin`, `member`, `viewer`. El contrato de permisos deberá aprobarse antes de crear invitaciones o mutaciones.

## Datos que necesita la primera vista

La pantalla solo necesita un resumen agregado, no credenciales de fabricante ni el historial completo:

- instalación: `id`, `name`, `timezone`;
- lecturas: `label`, `value` nullable, `unit`, `measuredAt`, `freshness` derivable;
- dispositivos: `id`, `name`, `kind`, `status` nullable;
- metadatos: `updatedAt` nullable y, si se acuerda, `source`/`quality`.

`measuredAt` y `updatedAt` deben ser ISO 8601 UTC. Un valor ausente se representa como `null`; el cliente mostrará `—` o «Sin datos disponibles» y nunca inferirá cero, apagado, ahorro o autonomía.

## Contratos OpenAPI propuestos

El contrato de diseño contiene únicamente:

- `GET /v1/me` para confirmar el sujeto y sus organizaciones autorizadas;
- `GET /v1/organizations/{organizationId}/sites` para listar instalaciones visibles;
- `GET /v1/organizations/{organizationId}/sites/{siteId}/overview` para la primera pantalla;
- `GET /v1/organizations/{organizationId}/sites/{siteId}/devices` para una futura pantalla dedicada.

Todos requieren `bearerAuth`. Las respuestas de error comparten `code`, `message`, `requestId` y detalles opcionales. No se añade un endpoint de telemetría ni mutaciones hasta acordar retención, permisos, origen de datos, paginación y límites.

Los modelos Go correspondientes están en `backend/internal/contracts/overview.go`. Sus pruebas de serialización cubren `null` para lecturas/fechas ausentes, el sobre de errores y las respuestas de listas. El paquete no registra rutas ni resuelve identidad: es una representación de contrato para la siguiente implementación.

La frontera de aplicación está en `backend/internal/site/service.go`: recibe un `Scope` con `organizationID` y `subject`, exige ambos antes de tocar el reader y obliga al puerto tenant-aware a recibir `organizationID` en cada operación. Sigue sin autenticar el subject; esa responsabilidad queda en el middleware futuro.

La conversión de una identidad ya verificada a ese scope está en `backend/internal/authz/scope.go`. `ScopeForOrganization` solo acepta una organización presente en las membresías del principal y rechaza subject ausente, organización inválida o pertenencia desconocida. No verifica JWT: la verificación concreta está en `backend/internal/authn/auth0.go`.

La extracción de credenciales está aislada en `backend/internal/authn/bearer.go`. `BearerToken` acepta exactamente un header Bearer y no registra ni valida su contenido; `Authenticate` conecta esa extracción con un `Verifier` inyectado; `Middleware` coloca únicamente el principal verificado en el contexto downstream y responde genéricamente ante errores. `Auth0Verifier` implementa ese puerto mediante discovery OIDC y JWKS cacheado por la librería. Solo extrae `sub`; las organizaciones y roles se resolverán en el servidor.

`Auth0Verifier` requiere `AUTH0_ISSUER` y `AUTH0_AUDIENCE` desde el entorno. `NewAuth0Verifier` todavía no se conecta a `main` ni a `NewRouter`, para no romper `/health` ni declarar rutas protegidas antes de disponer de configuración de despliegue y pruebas de autorización.

La composición HTTP base añade `X-Request-ID` generado por el servidor y lo conserva en el contexto mediante `backend/internal/platform/httpapi/request.go`. `WriteErrorResponse` ofrece el sobre `contracts.ErrorResponse` común para futuros handlers; todavía no hay endpoints de datos que lo utilicen.

## Puerta de entrada antes de datos reales

No se implementará el adaptador HTTP ni se sustituirá la fixture hasta disponer de:

1. valores de despliegue Auth0 para `issuer`/`audience` y una prueba contra el tenant real;
2. matriz de roles y política de pertenencia;
3. migraciones con claves y restricciones de organización;
4. contrato OpenAPI revisado y pruebas de contrato;
5. estrategia de errores, rate limiting, auditoría y observabilidad;
6. pruebas de autorización que intenten cruzar organizaciones;
7. decisión sobre retención y calidad de telemetría.

## Verificación contract-first

- RED: `go test ./internal/contracts` no compiló antes de crear los modelos.
- GREEN: `go test ./...` pasa; `go vet ./...` y `go build ./...` también pasan.
- Las pruebas están en `backend/internal/contracts/overview_test.go`.
- La frontera tenant-aware pasa `go test ./internal/site` con 84.6% de cobertura.
- La autorización de pertenencia pasa `go test ./internal/authz` con 100% de cobertura.
- La extracción Bearer pasa `go test ./internal/authn` con 100% de cobertura.
- El verifier Auth0 pasa pruebas con discovery/JWKS local TLS; cobertura total de `authn`: 95.7%.
- La base HTTP pasa `go test ./internal/platform/httpapi` con 96.6% de cobertura; la rama de fallo del generador aleatorio se prueba mediante seam inyectable.
- Los logs reproducibles están en `docs/evidence/backend-contract-first-2026-11-16/`.
