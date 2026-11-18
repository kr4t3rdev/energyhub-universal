# Evidencia — backend contract-first

Fecha: 2026-11-16 (actualizado 2026-11-18)

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `go mod verify`: PASS.
- `go test -race ./...`: PASS.
- Toolchain: Go 1.27.4, fijada en `backend/go.mod` y usada por CI mediante `go-version-file: backend/go.mod`.
- Integración en CI: el job de backend levanta un PostgreSQL 17 como servicio y ejecuta la suite `TestIntegration` con `TEST_DATABASE_URL`, así que el aislamiento de tenants y la frontera administrativa se comprueban en cada ejecución, no solo en la máquina del desarrollador.
- OpenAPI YAML parse: PASS (`contracts/openapi/next-phase-proposal.yaml`).
- RED/GREEN del paquete de contratos: `backend/internal/contracts/overview_test.go`.
- Frontera tenant-aware: `go test ./internal/site` PASS, cobertura de `site` 84.6%.
- Resolución de pertenencia: `go test ./internal/authz` PASS, cobertura de `authz` 100%.
- Autenticación Auth0/Bearer: `go test ./internal/authn` PASS, cobertura de `authn` 95.1%.
- Base HTTP: `go test ./internal/platform/httpapi` PASS, cobertura de `httpapi` 94.2%.
- Membresías PostgreSQL: `go test ./internal/storage/postgres` PASS, cobertura 98.1%, sin base real.
- RLS y admin: `go test ./internal/authz ./internal/storage/postgres` PASS; solo `SYSTEM_ADMIN_SUBJECT` llega al writer y la configuración vacía falla cerrada.
- Runner de migraciones: `go test ./internal/storage/postgres` PASS; `cmd/migrate` exige `MIGRATION_DATABASE_URL` (sin fallback a `DATABASE_URL`), rechaza el rol `energyhub_runtime` y no se ejecuta desde la API.
- Emisor OIDC local: `go test ./internal/devoidc ./cmd/devoidc` PASS, cobertura de `devoidc` 81.9%; emite tokens RS256, rechaza credenciales incorrectas y no sirve hosts que no sean loopback.
- Frontera admin por rol: `TEST_DATABASE_URL=... go test ./internal/storage/postgres -run TestIntegration` PASS contra PostgreSQL 17.11 real. La escritura admin ya no depende de `app.is_system_admin`, que era falsificable por el rol de la aplicación.
- Endurecimiento de la frontera admin (migración `000004`): PASS en la misma integración. El esquema `app` y las tablas de tenants pertenecen a `energyhub_owner`; la credencial admin no puede leer las tablas de tenants ni escribirlas directamente; la escritura exige a la vez el grupo `energyhub_membership_admin` y el rol `energyhub_admin_capability`; las escrituras quedan auditadas en `app.admin_audit_log`; crear una organización que ya existe es un conflicto explícito.
- Cierre del hueco de `TRUNCATE` (migración `000005`): PASS en la misma integración. `000004` no detectaba `TRUNCATE`, y el RLS no se aplica a `TRUNCATE`; `000005` revoca `INSERT`, `UPDATE`, `DELETE` y `TRUNCATE` sobre `organizations`, `organization_memberships` y `app.admin_audit_log` a todo rol que no herede `energyhub_owner` y falla cerrado si queda alguno. La propiedad de las funciones administrativas se reafirma sobre `energyhub_owner` y se comprueba.
- Pertenencia transitiva de roles: PASS en la misma integración. El bootstrap falla cerrado si un rol alcanza `energyhub_owner` o los grupos administrativos a través de una cadena de pertenencias (que no aparece en `pg_auth_members`) y si la credencial administrativa `energyhub_admin` se comparte con otro rol de login.
- Comprobación del rol de la API: PASS en la misma integración. `cmd/api` rechaza al arrancar una credencial superusuario, con `BYPASSRLS` o con cualquier privilegio de mutación sobre las tablas protegidas, incluida la traza de auditoría.
- Rollback de migraciones: PASS en la misma integración. El `down` de `000004` restaura el estado de `000003` y el `down` de `000003` retira las funciones admin conservando la lectura.
- Migración con rol no superusuario: PASS. `cmd/api` verifica al arrancar que su credencial no es superusuario, no tiene `BYPASSRLS` y no puede escribir.
- Limitación conocida y fijada por prueba: `TestIntegrationDocumentsForgeableTenantContext` PASS. El GUC `app.current_subject` es falsificable, así que la credencial de base de datos es la frontera de confianza de la lectura.
- Límites de recursos HTTP: PASS. `cmd/api` y `cmd/devoidc` fijan `ReadHeaderTimeout` 10 s, `ReadTimeout` 15 s, `WriteTimeout` 30 s, `IdleTimeout` 60 s y `MaxHeaderBytes` 8 KiB; el cliente OIDC y el discovery tienen timeout de 10 s.
- Cabeceras de respuesta: PASS. La ruta protegida responde `Cache-Control: no-store` y `Vary: Authorization` en el camino de éxito y en los errores (`401` y `500`): las cabeceras se fijan antes de la autenticación.
- Emisor de desarrollo endurecido: PASS. Solo acepta una IP literal de loopback, comprueba el par remoto (`RemoteAddr`), limita el cuerpo de la petición de token a 16 KiB, rechaza el contenido posterior al primer objeto JSON, responde `413` a un cuerpo demasiado grande, marca la respuesta de token con `no-store` y no imprime secretos al arrancar. La API se niega a arrancar con el emisor de desarrollo si `LISTEN_ADDR` no es loopback.
- Migrador: PASS. `cmd/migrate` exige `MIGRATION_DATABASE_URL`, rechaza el rol `energyhub_runtime` y comprueba el rol effective de la sesión (`current_user`), no solo el userinfo de la URL.
- Redaction de credenciales: PASS. `cmd/api`, `cmd/membership` y `cmd/migrate` quitan la contraseña del DSN de sus errores de conexión antes de mostrarlos, porque el driver de PostgreSQL incluye la cadena original sin filtrar en el error de una URL mal formada y ese error llega al log de arranque.
- Higiene del cluster de pruebas: PASS. La suite de integración libera los privilegios del rol temporal en la base de prueba antes de eliminarlo y deja el cluster sin roles ni bases residuales, comprobado con `pg_roles` y `pg_database` después de la ejecución.
- Stack local endurecido: `backend/scripts/dev-stack.sh` usa variables de `psql` en vez de interpolación, valida el nombre de la base y las contraseñas, no entrega `ADMIN_DATABASE_URL` al proceso API, escucha en `127.0.0.1:8080`, escribe el log del emisor con `mktemp` y `umask 077` y verifica la readiness del emisor.
- Ruta protegida y CLI admin: `go test ./cmd/api ./cmd/membership ./internal/platform/httpapi` PASS; 401 sin token o con token manipulado, 200 con las organizaciones del sujeto y ningún detalle interno en las respuestas de error.
- Flujo de extremo a extremo verificado contra PostgreSQL 17.11 y el emisor local: ver `local-stack-e2e.md`.
- El verifier Auth0 valida firma y claims estándar mediante discovery/JWKS y ya está conectado al router protegido; las organizaciones y roles se resuelven en PostgreSQL, nunca desde el token.
- No hay pruebas en teléfonos físicos: el alcance de esta evidencia es backend, PostgreSQL local y emisor OIDC de desarrollo.
