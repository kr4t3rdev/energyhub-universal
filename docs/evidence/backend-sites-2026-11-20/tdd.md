# Endpoints de sitios y dispositivos — TDD y evidencia — 2026-11-20

Implementación de los 4 endpoints de `contracts/openapi/next-phase-proposal.yaml`
(`GET /v1/me`, `GET /v1/organizations/{organizationId}/sites`,
`GET /v1/organizations/{organizationId}/sites/{siteId}/overview`,
`GET /v1/organizations/{organizationId}/sites/{siteId}/devices`) siguiendo el
contrato exactamente: paths, operationIds, esquemas y sobre de error
`{code, message, requestId}`. La propuesta sigue siendo un artefacto de diseño;
no se tocó `contracts/` ni `apps/` ni `tasks.md` de openspec.

## Capas y commits (locales, sin push)

- `48bd01d` — migración `000007` (tablas `sites` y `site_devices`, RLS
  habilitado y forzado con políticas por membresía, propiedad
  `energyhub_owner`, runtime solo SELECT, funciones SECURITY DEFINER
  `app.create_site` / `app.create_site_device` con comprobación de capacidad y
  auditoría, revocación de privilegios sobrantes incluido TRUNCATE) + pruebas
  de contrato de migraciones.
- `faecfd7` — reader Postgres `internal/storage/postgres/site.go` + threading
  del subject verificado por el servicio de `internal/site`.
- `cf2b5bc` — handlers `internal/platform/httpapi/sites.go`, opción de router
  `WithSites` y cableado en `cmd/api`.
- `78ba01b` — `gofmt` en el fake del test del servicio de sitios.
- `597f96f` — CLI de admin: `AdminSiteService` en `internal/authz`,
  `SiteWriter` en `internal/storage/postgres` y acciones `create-site` /
  `create-device` en `cmd/membership` (UUID v4 con stdlib `crypto/rand`,
  sin `google/uuid` en `go.mod`).
- `e7fa732` — tests de integración de sitios + corrección del orden de
  argumentos del writer (ver abajo).

## TDD por capa (RED observado antes de cada GREEN)

1. **Migración:** pruebas de contrato primero; RED (ficheros no encontrados);
   GREEN tras escribir `000007` up/down.
2. **Reader + servicio:** tests del reader primero; RED dos veces (fallo de
   compilación por el fake con la firma antigua, luego `NewSiteReader`
   indefinido); GREEN: 12/12 tests del reader, `internal/site` en verde.
3. **Handlers:** `sites_test.go` primero; RED (`WithSites` indefinido); GREEN:
   16 tests de rutas en verde, incluido que `/v1/organizations` sigue
   funcionando con los sitios habilitados y que las rutas no se registran sin
   `WithSites`.
4. **CLI:** tests primero en los tres paquetes; RED (símbolos indefinidos en
   `authz`, `postgres` y `cmd/membership`); GREEN tras implementar el servicio
   de admin, el writer y el parseo/ejecución de las acciones.
5. **Integración:** el test nuevo se escribió antes de ejecutarlo contra la
   base real. La primera ejecución falló en dos puntos reales (ver abajo) y
   quedó en verde tras corregir el writer y el test.

## Desviaciones y correcciones (documentadas, no reabiertas)

1. **Threading del subject por el reader (desviación de diseño).** El `Reader`
   de `internal/site` solo llevaba `organizationID`, pero el reader Postgres
   debe fijar `app.current_subject` en su transacción como el lector de
   membresías. `Scope` ya lleva `Subject`, así que los tres métodos del
   `Reader` ganaron un parámetro `subject` y el servicio pasa `scope.Subject`.
   El servicio mantiene validación de ámbito, delegación y propagación de
   errores sin cambios, y sigue sin mapear `ErrSiteNotFound`. La alternativa
   (leer el principal del contexto de la petición dentro de storage) se
   rechazó por implícita y no testeable.
2. **Orden de argumentos de las funciones admin (corrección, no desviación
   del contrato).** La integración reveló que `app.create_site` y
   `app.create_site_device` toman el id de la entidad creada primero
   (`create_site(site_id, organization_id, ...)`,
   `create_site_device(device_id, site_id, organization_id, ...)`), mientras
   que el writer pasaba la organización primero; PostgreSQL rechazaba la
   llamada con `invalid input syntax for type uuid: "org-1"`. La migración
   `000007` es forward-only y no se tocó: el writer ahora llama a las
   funciones en su orden declarado. La interfaz `SiteWriter` y el servicio
   mantienen la organización primero (convención del dominio); solo cambia el
   orden de los argumentos SQL dentro del writer.
3. **Columna de auditoría.** El test de integración consultaba
   `app.admin_audit_log.action`; la columna real es `operation` (migración
   `000004`). Corregido el test.

## Decisiones de diseño (ya tomadas, aplicadas)

- Creación de sitios/dispositivos solo por CLI de admin (`create-site` /
  `create-device` con la credencial admin); sin superficie HTTP de admin.
- Sin tabla de lecturas en esta tranche: `GetOverview` devuelve
  `readings: []` y `updatedAt: null`.
- `GET /v1/me` devuelve el mismo payload `CurrentUser` que
  `GET /v1/organizations`; ambas rutas siguen vivas.
- Errores sin filtrar existencia: no miembro → 403; sitio no visible o
  inexistente (incluido UUID malformado) → 404; sin token o inválido → 401;
  resto → 500 genérico con el sobre común. `site.ErrSiteNotFound` lo devuelve
  el reader Postgres y el handler lo mapea con `errors.Is`. La comprobación
  de membresía (403) ocurre antes de buscar el sitio (404).

## Verificación final (Mac, Go 1.27.4, `backend/`)

- `go test ./... -count=1`: 11 paquetes ok, **297 casos en verde, 0 fallos**
  (incluye subtests).
- `go vet ./...` y `go build ./...`: OK.
- `go test -race ./... -count=1`: 11 paquetes ok.
- Integración real (`TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/postgres`,
  PostgreSQL 17.6 local): **15/15 en verde**, incluido el test nuevo
  `TestIntegrationSiteTenantIsolationAndAdminWrites` (11 subtests: creación
  por admin, auditoría, aislamiento por tenant, overview sin lecturas,
  404 para sitio ajeno/desconocido/malformado, dispositivos propios,
  runtime sin escritura directa, runtime sin llamar a las funciones admin,
  admin sin lectura de las tablas de tenants, cascada al borrar la
  organización) y el subtest nuevo de rollback de `000007` dentro de
  `TestIntegrationDownMigrationsExecute`.
- Logs: `logs/unit-tests.log`, `logs/vet-build.log`, `logs/race.log`,
  `logs/integration.log`, `logs/e2e.log`.

## E2E (stack local, 2026-11-20)

Stack con `scripts/dev-stack.sh` (PGPORT=5432, base `energyhub_dev`,
migraciones 000001–000007), orgs `org-e2e-sites` / `org-e2e-other`, sitio y
dispositivo creados por CLI, token del issuer de desarrollo como `user1`
(`dev|user-1`), API en `127.0.0.1:8080` sin credencial admin. **12/12 checks
en verde** (detalle y cuerpos en `logs/e2e.log`): 200 con datos reales en
los 4 endpoints, 200 en la ruta existente `/v1/organizations`, 401 sin token
y con token manipulado, 403 con otra organización, 404 con sitio ajeno,
inexistente y malformado (overview y devices).

Notas del entorno: `psql` y `go` no están en el PATH de las sesiones SSH no
interactivas; hay que exportar `/opt/homebrew/bin`. La primera arrancada de
la API falló porque `LISTEN_ADDR` por defecto es `:8080` y el issuer de
desarrollo en HTTP plano exige loopback explícito (`LISTEN_ADDR=127.0.0.1:8080`,
como hace `dev-stack.sh --with-api`). El proceso de `dev-stack.sh` quedó
colgado en la sesión SSH por el pipe de salida retenido por el issuer en
segundo plano; el stack quedó arriba igualmente (env en `.local/dev-stack.env`
modo 0600, issuer sirviendo). La API del E2E se paró al terminar; el issuer
quedó corriendo, como lo deja `dev-stack.sh`.

## Pendiente (fuera de esta tranche)

- Telemetría / lecturas reales (`readings: []`, `updatedAt: null` hasta que
  exista origen de datos).
- Conectar las 3 apps (web, Android, iOS) a la API real en vez de la fixture.
- Pruebas en teléfono físico (iOS/Android).
- Push de los 6 commits locales (lo hace el usuario desde su Terminal).
