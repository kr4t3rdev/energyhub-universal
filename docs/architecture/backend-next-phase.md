# Diseño de la siguiente fase backend

Estado: diseño aprobado e implementado en el alcance de membresías. Hay un verifier Auth0 conectado a una ruta protegida (`GET /v1/organizations`) y acceso a PostgreSQL con dos credenciales separadas (runtime y admin). La frontera de escritura administrativa está reforzada con una migración adicional (`000004`), el servidor HTTP tiene límites de recursos y el emisor OIDC de desarrollo está restringido a loopback. No hay endpoints de dispositivos, telemetría ni datos reales.

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

La frontera de aplicación está en `backend/internal/site/service.go`: recibe un `Scope` con `organizationID` y `subject`, exige ambos antes de tocar el reader y obliga al puerto tenant-aware a recibir `organizationID` en cada operación. El servicio no verifica el JWT; esa responsabilidad está en `backend/internal/authn/middleware.go` y en la composición HTTP.

La conversión de una identidad ya verificada a ese scope está en `backend/internal/authz/`. `MembershipReader` resuelve organizaciones por `subject` desde una fuente server-side; `MembershipAuthorizer` entrega esas membresías a `ScopeForOrganization`, que rechaza subject ausente, organización inválida o pertenencia desconocida. No verifica JWT: la verificación concreta está en `backend/internal/authn/auth0.go`.

La extracción de credenciales está aislada en `backend/internal/authn/bearer.go`. `BearerToken` acepta exactamente un header Bearer y no registra ni valida su contenido; `Authenticate` conecta esa extracción con un `Verifier` inyectado; `Middleware` coloca únicamente el principal verificado en el contexto downstream y responde genéricamente ante errores. `Auth0Verifier` implementa ese puerto mediante discovery OIDC y JWKS cacheado por la librería. Solo extrae `sub`; las organizaciones y roles se resolverán en el servidor.

`Auth0Verifier` requiere `AUTH0_ISSUER` y `AUTH0_AUDIENCE` desde el entorno. `cmd/api` lo instancia solo cuando hay issuer, audience y `DATABASE_URL`; sin esa configuración el proceso expone únicamente `/health` y no registra ninguna ruta protegida.

La composición HTTP base añade `X-Request-ID` generado por el servidor y lo conserva en el contexto mediante `backend/internal/platform/httpapi/request.go`. `WriteErrorResponse` ofrece el sobre `contracts.ErrorResponse` común; la ruta protegida `/v1/organizations` ya lo usa, también en los errores de autenticación, siempre con `X-Request-ID` y sin detalles internos.

## Membresías en PostgreSQL

`backend/internal/storage/postgres/membership.go` implementa `authz.MembershipReader` sobre PostgreSQL:

- `ListOrganizations` recibe el `subject` ya verificado y lo enlaza como `$1`; no concatena valores en SQL.
- La consulta une `organization_memberships` con `organizations` y ordena por nombre e identificador.
- El rol se valida contra el contrato (`owner`, `admin`, `member`, `viewer`) y un valor inesperado en la base produce error en lugar de degradar permisos.
- Las filas se cierran siempre y el resultado vacío se devuelve como lista no nula.
- `DB`/`Tx`/`Rows` son puertos estrechos con un adaptador `NewDB` para `pgxpool.Pool`; las pruebas usan fakes locales, sin depender de una base real.

`backend/migrations/000001_create_organizations_and_memberships.up.sql` crea `organizations` y `organization_memberships` con `organization_id NOT NULL`, clave primaria `(organization_id, subject)`, `ON DELETE CASCADE` e índice por `subject`. El `CHECK` de roles se contrasta con el contrato mediante una prueba que lee la migración.

El runner explícito está en `backend/cmd/migrate` y `backend/internal/storage/postgres/migrate.go`. `LoadMigrations` valida nombres, cuerpos, duplicados y orden; `Migrate` crea `schema_migrations`, omite versiones ya aplicadas y se niega a continuar si la base tiene una versión desconocida para el binario. `PgxMigrationStore` ejecuta el SQL de cada archivo con protocolo simple (para admitir varias sentencias) y registra la versión en la misma transacción. La API no ejecuta migraciones; solo se aplican con `MIGRATION_DATABASE_URL=... go run ./cmd/migrate`.

`MIGRATION_DATABASE_URL` es obligatoria y no tiene fallback a `DATABASE_URL`: la credencial de la aplicación es de solo lectura y no puede ser la que migra. `cmd/migrate` rechaza explícitamente el rol `energyhub_runtime` y, además, comprueba el rol effective de la sesión abierta (`current_user`) antes de migrar: el userinfo de la URL es solo un diagnóstico previo, porque la conexión puede resolver el rol desde el entorno o desde la configuración del servidor. Las migraciones son forward-only una vez aplicadas: `backend/migrations/README.md` prohíbe editar una migración ya aplicada, y por eso el endurecimiento posterior se añadió como `000004` y `000005` y no como una edición de `000003`.

## RLS y administración

`backend/migrations/000002_enable_tenant_rls.up.sql` habilita y fuerza RLS en `organizations` y `organization_memberships`:

- `app.current_subject` limita la lectura a las membresías propias y a las organizaciones donde el subject es miembro.
- El lector fija `app.current_subject` en la misma transacción de la consulta, a partir del subject ya verificado.

`backend/migrations/000003_secure_admin_writes.up.sql` sustituye la escritura administrativa basada en `app.is_system_admin` por una frontera de rol de PostgreSQL. La comprobación empírica contra PostgreSQL 17.11 mostró que un GUC personalizado no puede protegerse (`REVOKE SET ON PARAMETER` no aplica a `app.*`): con la credencial de la aplicación era posible ejecutar `SET app.is_system_admin = 'on'` y escribir membresías sin pasar por la autorización del servidor.

`backend/migrations/000004_harden_admin_role_boundary.up.sql` cierra los huecos que quedaban en `000003`:

- La propiedad del esquema `app` y de las tablas de tenants pasa al rol `energyhub_owner` (`NOLOGIN`, `BYPASSRLS`). Un rol que sea el propietario de una tabla puede desactivarle el RLS, así que la propiedad deja de estar en un rol de aplicación.
- La credencial admin pierde el `SELECT` global sobre las tablas de tenants: las políticas de lectura ya no conceden acceso al bypass de administrador. El rol admin no puede leer los datos de ningún tenant.
- La escritura sigue siendo solo por funciones `SECURITY DEFINER`, pero la habilitación exige dos cosas distintas: pertenecer al grupo `energyhub_membership_admin` (que es quien tiene el `EXECUTE`) y pertenecer al rol `energyhub_admin_capability` (NOLOGIN), que las funciones comprueban por dentro con `pg_has_role(session_user, ...)`. Conceder uno de los dos por error no abre la puerta.
- Las escrituras administrativas quedan auditadas en `app.admin_audit_log`, que registra el actor (`session_user`), la operación y el objetivo.
- `app.create_organization` devuelve un conflicto explícito cuando la organización ya existe, en lugar de renombrarla en silencio.
- El script falla cerrado si un rol inesperado entra en los grupos administrativos.

`backend/migrations/000005_revoke_leftover_protected_table_privileges.up.sql` cierra el hueco que quedó en `000004`:

- La detección de privilegios sobrantes de `000004` no incluía `TRUNCATE`, y el RLS no se aplica a `TRUNCATE`: un rol con solo ese privilegio podía borrar las tablas de tenants. `000005` revoca `INSERT`, `UPDATE`, `DELETE` y `TRUNCATE` sobre `organizations`, `organization_memberships` y `app.admin_audit_log` a todos los roles que no heredan `energyhub_owner`, y falla cerrado si queda alguno.
- La propiedad de las funciones administrativas se vuelve a afirmar sobre `energyhub_owner` y se comprueba. `CREATE OR REPLACE` conserva el propietario anterior, así que sin esta afirmación una restauración de la base podía dejar las funciones en manos del rol que las instaló, que podría reemplazar su cuerpo y quitarse la comprobación de capacidad.
- `000005` es una migración nueva y no una edición de `000004`: una migración ya aplicada no se edita. El `down` de `000004` archiva la traza de auditoría en `public.admin_audit_log_archive_000004` antes de borrar la tabla, para que una reversión manual no borre la evidencia.

El bootstrap (`backend/deploy/postgres/bootstrap.sql`) resuelve la pertenencia de roles de forma transitiva, no solo directa. Una cadena como `GRANT energyhub_owner TO helper; GRANT helper TO energyhub_runtime` no aparece en `pg_auth_members`, pero concede a la credencial de la API los derechos de propietario sobre las tablas, así que el bootstrap falla cerrado en ese caso. El mismo control cubre `energyhub_admin`: compartir la credencial administrativa con otro rol de login crearía una segunda credencial de escritura completa. Un superusuario es miembro de todos los roles, así que no cuenta como una credencial compartida.

El modelo actual tiene dos credenciales y dos fronteras:

- `DATABASE_URL` usa el rol `energyhub_runtime`, que solo tiene `SELECT`; las políticas RLS limitan la lectura al subject verificado y a las organizaciones donde es miembro.
- `ADMIN_DATABASE_URL` usa el rol `energyhub_admin`, miembro del grupo `energyhub_membership_admin` y del rol `energyhub_admin_capability`. Solo ese par habilita las funciones `SECURITY DEFINER` `app.create_organization`, `app.grant_membership` y `app.revoke_membership`, que son la única vía de escritura.
- No existe ninguna política de escritura directa: incluso el rol admin recibe `permission denied` al insertar en las tablas.

`SYSTEM_ADMIN_SUBJECT` es el único subject autorizado a crear, actualizar o revocar membresías. Está vacío por defecto y la autorización falla cerrada. `AdminMembershipService` valida organización, subject objetivo y rol antes de llamar al puerto; el writer vuelve a comparar el admin configurado antes de abrir la transacción, y no se invoca para un principal que no sea el admin.

El límite real de esta frontera es la credencial de PostgreSQL, no el valor de `SYSTEM_ADMIN_SUBJECT`: quien tenga el rol admin puede escribir membresías sin pasar por la comprobación del subject. `SYSTEM_ADMIN_SUBJECT` es una segunda comprobación dentro de la aplicación, no una frontera independiente.

Al arrancar, `cmd/api` verifica la credencial que va a usar con `postgres.RuntimeRoleVerifier`: rechaza el rol si es superusuario, si tiene `BYPASSRLS`, si puede modificar `organizations` o `organization_memberships` con `INSERT`, `UPDATE`, `DELETE` o `TRUNCATE` (el RLS no se aplica a `TRUNCATE`), o si tiene acceso al esquema `app`, que contiene las funciones administrativas y la traza de auditoría. La traza no se consulta directamente porque el rol no puede ni resolver su nombre: el control es la ausencia de acceso al esquema. Un despliegue con la credencial equivocada falla al iniciar en lugar de exponer lectura sin aislamiento.

`cmd/api` conecta `NewMembershipReader` con el pool runtime para la ruta protegida y `cmd/membership` conecta `NewMembershipWriter` y `NewOrganizationWriter` con el pool admin para la administración por CLI. El aislamiento está verificado contra una instancia PostgreSQL real: la prueba `TestIntegrationTenantIsolationAndAdminWrites` crea los roles, aplica las migraciones y comprueba que el rol runtime no escribe, no llama a las funciones admin, no ve otras membresías y que el rol admin escribe solo a través de las funciones (incluida la creación de organizaciones), que la creación repetida de una organización es un conflicto y que las escrituras quedan auditadas. Se activa con `TEST_DATABASE_URL` apuntando a un superusuario.

### Limitación conocida: el contexto de tenant en lectura

`app.current_subject` sigue siendo un GUC de sesión, así que quien tenga la credencial runtime puede fijar un subject arbitrario y leer los datos de otro tenant. A diferencia de `app.is_system_admin`, este GUC no se usa para escribir, sino solo para filtrar lecturas.

La consecuencia honesta es que la credencial de base de datos es la frontera de confianza de la lectura, no el subject. Mitigarlo exige mover el aislamiento a roles de PostgreSQL por tenant (un rol por tenant, o un `SET ROLE` a un rol derivado del subject verificado), lo que cambia el modelo de despliegue. Esa decisión está pendiente y no se ha implementado.

Mientras tanto, el comportamiento está fijado por una prueba explícita: `TestIntegrationDocumentsForgeableTenantContext` falla si la falsificación deja de ser posible, de modo que el cambio no puede ocurrir por accidente ni quedar sin registrar.

## Composición HTTP y ruta protegida

`backend/cmd/api/main.go` compone el servidor. `loadAPIConfig` exige autenticación y base de datos en bloque: una sin la otra es un error de configuración, no un modo degradado. Sin configuración solo se registra `/health`, que sigue siendo liveness puro.

`httpapi.NewRouter` acepta opciones. `WithAuthentication(verifier, memberships)` registra `/v1/organizations` detrás de `authn.Middleware`; si falta el verificador o el reader, la ruta no se registra y no queda superficie protegida expuesta. El handler toma el sujeto del principal verificado, resuelve las organizaciones en PostgreSQL y devuelve `contracts.CurrentUser`. Los errores de autenticación usan el mismo sobre `contracts.ErrorResponse` que el resto de la API, con `X-Request-ID` y sin detalles internos.

## Límites de recursos HTTP

El servidor de `cmd/api` no acepta peticiones sin límites: `ReadHeaderTimeout` 10 s, `ReadTimeout` 15 s, `WriteTimeout` 30 s, `IdleTimeout` 60 s y `MaxHeaderBytes` 8 KiB. El mismo conjunto de límites se aplica al emisor OIDC de desarrollo.

Las respuestas de la ruta protegida dependen del sujeto verificado, así que se marcan `Cache-Control: no-store` y `Vary: Authorization`. Las cabeceras se fijan antes de la autenticación, de modo que los sobres de error, incluidos el `401` y el `500` genérico, también los llevan y ningún error autenticado queda en una caché compartida.

El cliente OIDC usa un timeout de 10 s y el discovery se acota con su propio timeout de 10 s, de modo que un emisor lento no bloquea el arranque ni una petición indefinidamente. Si se entrega un cliente HTTP propio sin timeout, se clona con uno, porque la renovación del conjunto de claves descarta el plazo del contexto y el timeout del cliente es el único límite que le queda.

El API no termina TLS: el despliegue lo resuelve delante del proceso (terminador TLS o proxy inverso). El proceso solo debe escuchar en una interfaz interna, y nunca con la credencial administrativa en su entorno.

Las tres herramientas que abren una conexión (`cmd/api`, `cmd/membership` y `cmd/migrate`) reescriben el error de conexión antes de mostrarlo, quitando la contraseña del DSN. No es cosmético: el driver de PostgreSQL incluye la cadena de conexión original, sin filtrar, en el error de una URL mal formada, y ese error acaba en el log de arranque. La credencial de migración es la más sensible del sistema, así que su redaction no es opcional.

## Emisor OIDC de desarrollo

`backend/cmd/devoidc` es una herramienta de desarrollo y está restringido a loopback:

- `DEV_OIDC_ADDR` solo acepta una IP literal de loopback. `localhost` se rechaza porque depende de DNS y del archivo `hosts`, así que no prueba que el tráfico se quede en la máquina.
- La comprobación del par remoto usa `RemoteAddr` y no solo el `Host`, que elige el cliente.
- El cuerpo de la petición de token se limita a 16 KiB, se decodifica con `DisallowUnknownFields` y no admite contenido detrás del primer objeto JSON. Una petición demasiado grande se responde con `413`.
- La respuesta de token lleva `Cache-Control: no-store`.
- El arranque no imprime el secreto del cliente ni las contraseñas: solo indica en qué variable de entorno está cada valor, y el ejemplo de `curl` las lee del entorno con su valor de desarrollo por defecto.

La misma regla se aplica al lado de la API: un issuer OIDC sobre HTTP plano solo se acepta con una IP literal de loopback, y en ese caso la API se niega a arrancar si `LISTEN_ADDR` no es una dirección de loopback. Así, las credenciales de desarrollo no pueden quedar expuestas a otra máquina.

## Administración por CLI

`backend/cmd/membership` es la única vía para crear organizaciones y conceder o revocar membresías:

```sh
ADMIN_DATABASE_URL=... SYSTEM_ADMIN_SUBJECT=dev|admin \
  go run ./cmd/membership create-organization org-1 "Demo Org"
ADMIN_DATABASE_URL=... SYSTEM_ADMIN_SUBJECT=dev|admin \
  go run ./cmd/membership grant org-1 dev|user-1 viewer
```

El CLI actúa como el admin del sistema configurado y usa la credencial administrativa, que en PostgreSQL solo puede ejecutar las funciones `app.*`. No hay superficie HTTP que conceda membresías.

## Stack local

Mientras no exista un tenant Auth0, `backend/scripts/dev-stack.sh` levanta el entorno de desarrollo: roles de PostgreSQL, migraciones, emisor OIDC local y las variables de entorno para la API. `backend/cmd/devoidc` arranca el emisor en loopback con un usuario admin precreado (`dev|admin`); `backend/dev.env.example` documenta sus variables. Cuando exista el tenant real, solo cambian `AUTH0_ISSUER`, `AUTH0_AUDIENCE` y `SYSTEM_ADMIN_SUBJECT`: el verificador no cambia.

El script trata las credenciales de desarrollo como credenciales:

- Los valores que llegan a SQL se pasan como variables de `psql` (`:'nombre'` para literales, `:"nombre"` para identificadores), nunca por interpolación en la cadena, y el nombre de la base, el host, el puerto, el usuario y las contraseñas se validan contra un conjunto de caracteres restringidos antes de usarse. Las contraseñas se escriben desde el entorno con `\getenv`, así que tampoco aparecen en la lista de procesos.
- El proceso API nunca recibe `ADMIN_DATABASE_URL` ni `MIGRATION_DATABASE_URL`: la credencial administrativa es solo del CLI y el script las elimina del entorno heredado de los dos procesos que lanza.
- El DSN del runtime se exporta al proceso API en vez de pasarse como argumento, así que tampoco aparece en la lista de procesos: el entorno del proceso lo necesita, la lista de procesos no.
- La API del stack local escucha en `127.0.0.1:8080`.
- El log del emisor se crea con `mktemp` y con `umask 077`, y el script espera a que el emisor responda antes de continuar. Las comprobaciones de readiness usan `curl` con timeout, así que un proceso que acepta la conexión pero no responde no bloquea el script.
- El script comprueba que `psql` y `curl` estén disponibles y falla con un mensaje claro si no lo están.

Las credenciales del stack local son valores de desarrollo fijos en el repositorio: sirven para el entorno local y no para ningún otro sitio.

## Rotación de credenciales

La frontera de escritura depende de la credencial de PostgreSQL, así que su rotación es parte de la seguridad y no una tarea de operación opcional:

- Las dos credenciales nunca se comparten: `DATABASE_URL` (rol `energyhub_runtime`, solo lectura) vive en el proceso API; `ADMIN_DATABASE_URL` (rol `energyhub_admin`) vive solo en el entorno desde el que se ejecuta `cmd/membership`. El proceso API no lee la credencial administrativa.
- La credencial admin no se entrega a procesos de larga vida ni a servicios que atiendan peticiones de usuarios: la administración es una operación puntual de consola.
- Rotación del rol admin: `ALTER ROLE energyhub_admin PASSWORD ...` y actualización del secreto del CLI en el mismo cambio. PostgreSQL conserva una sola contraseña por rol, así que al cambiarla la anterior queda invalidada de inmediato; no hay ventana con dos contraseñas válidas.
- Rotación del rol runtime: mismo procedimiento y reinicio de la API, que vuelve a hacer el ping al arrancar. No hay escrituras en la base que puedan quedar a medias, porque la API nunca escribe.
- `SYSTEM_ADMIN_SUBJECT` no cambia al rotar credenciales: es el `sub` del usuario admin del sistema en el proveedor OIDC. Solo cambia si el proveedor emite otro subject para la cuenta admin.
- Ninguna credencial se escribe en el repositorio ni en un `.env` versionado; el gestor de secretos es responsabilidad del despliegue y cada entorno entrega sus propios valores.
- La cadencia y el procedimiento los fija la operación del despliegue; la recomendación por defecto es rotar cada 90 días y de inmediato ante cualquier sospecha de exposición. La rotación no cambia la frontera: sigue siendo la pertenencia al grupo `energyhub_membership_admin` y al rol `energyhub_admin_capability` la que habilita las funciones de escritura.

## Puerta de entrada antes de datos reales

No se implementará el adaptador HTTP ni se sustituirá la fixture hasta disponer de:

1. valores de despliegue Auth0 para `issuer`/`audience` y una prueba contra el tenant real;
2. matriz de roles y política de pertenencia;
3. migraciones con claves y restricciones de organización; el esquema inicial, el runner y las políticas RLS existen y están verificados contra PostgreSQL real;
4. contrato OpenAPI revisado y pruebas de contrato;
5. estrategia de errores, rate limiting, auditoría y observabilidad; el sobre de errores y la auditoría de escrituras administrativas ya existen, pero el rate limiting por IP o por subject sigue pendiente;
6. pruebas de autorización que intenten cruzar organizaciones; el aislamiento de lectura está verificado contra PostgreSQL real y la falsificación del contexto de tenant en lectura está documentada como limitación conocida;
7. decisión sobre retención y calidad de telemetría;
8. decisión sobre el modelo de aislamiento de lectura (roles por tenant frente al GUC de sesión), porque la credencial de base de datos es hoy la frontera de confianza de la lectura.

## Verificación contract-first

- RED: `go test ./internal/contracts` no compiló antes de crear los modelos.
- GREEN: `go test ./...` pasa; `go vet ./...` y `go build ./...` también pasan.
- Las pruebas están en `backend/internal/contracts/overview_test.go`.
- La frontera tenant-aware pasa `go test ./internal/site` con 84.6% de cobertura.
- La autorización de pertenencia pasa `go test ./internal/authz` con 100% de cobertura.
- La extracción Bearer pasa `go test ./internal/authn`; cobertura total del paquete `authn`: 96.2%.
- El verifier Auth0 pasa pruebas con discovery/JWKS local TLS y contra el emisor del paquete `internal/devoidc`.
- La base HTTP pasa `go test ./internal/platform/httpapi` con 93.9% de cobertura; la rama de fallo del generador aleatorio se prueba mediante seam inyectable.
- El lector PostgreSQL de membresías pasa `go test ./internal/storage/postgres` con 100% de cobertura, sin base real ni red.
- RLS y administración pasan `go test ./internal/authz ./internal/storage/postgres` con 100% de cobertura: solo `SYSTEM_ADMIN_SUBJECT` llega al writer, fallo cerrado sin configuración, y el SQL RLS se contrasta leyendo la migración.
- El emisor OIDC local pasa `go test ./internal/devoidc ./cmd/devoidc` con 82.0% de cobertura del paquete `devoidc`.
- Verificación adicional del estado actual: `go test -race ./...` en verde y la integración real `TEST_DATABASE_URL=... go test ./internal/storage/postgres -run TestIntegration` en verde contra PostgreSQL 17.11. La suite de integración cubre el aislamiento de tenants, la frontera administrativa (11 subcasos), la auditoría, el conflicto de creación duplicada, el rollback de `000004` y `000003`, la propiedad de los objetos en `energyhub_owner` y la migración ejecutada por un rol no superusuario.
- El runner de migraciones pasa las mismas pruebas con 100% de cobertura: validación de nombres/orden/duplicados, transacciones, rollback y rechazo de versiones desconocidas. No se ejecutó contra una base real.
- La toolchain del backend está fijada en Go 1.27.4 (`backend/go.mod`), que es la versión que usa CI porque lee `go-version-file: backend/go.mod`.
- La suite de integración también corre en CI: el job de backend levanta un PostgreSQL 17 como servicio y ejecuta `go test ./internal/storage/postgres -run TestIntegration` con `TEST_DATABASE_URL` apuntando a ese servicio. Sin ese paso, el aislamiento de tenants y la frontera administrativa solo se probarían en la máquina del desarrollador, y una regresión de permisos llegaría al despliegue sin que nada la detuviera.
- Los logs reproducibles están en `docs/evidence/backend-contract-first-2026-11-16/`.
