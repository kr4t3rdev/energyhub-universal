# Diseño — Emisor OIDC local y frontera de escritura admin

Fecha: 2026-11-17
Estado: aprobado por el usuario (frontera de admin: doble credencial + `SECURITY DEFINER`) e implementado en `1af7f21`, `568266f`, `5172f27` y `ca13558`. Pendiente: la prueba contra el tenant Auth0 real.

## Contexto

No existe todavía un tenant Auth0. El usuario pidió probar el flujo de autenticación con un emisor
propio y dejar la prueba real para cuando se pueda crear el tenant y el usuario admin.

Al verificar las políticas RLS contra PostgreSQL 17.11 real (Homebrew, puerto 55432) apareció un
defecto en el modelo entregado en `618ba45`: el rol de la aplicación puede ejecutar
`SET app.is_system_admin='on'` por su cuenta. Los GUC personalizados no admiten
`REVOKE SET ON PARAMETER`, así que un GUC no puede ser la frontera de privilegio. Consecuencia:
cualquiera que tenga la credencial de BD —incluido un SQL inyectado en la API— podía declararse
admin y escribir membresías.

Verificado empíricamente en PostgreSQL 17.11:

- rol runtime: `INSERT` directo → `permission denied for table organizations` (RLS + privilegios).
- rol runtime: `SET app.is_system_admin='on'` → permitido (falso boundary).
- rol admin vía función `SECURITY DEFINER` de un dueño `BYPASSRLS` → escritura permitida.
- rol runtime: `EXECUTE` de esa función → `permission denied for function` (tras `REVOKE` a `PUBLIC`).
- lectura con `app.current_subject='dev|user-b'` → 0 filas (aislamiento de lectura funciona).

## Objetivo

1. Un emisor OIDC local ejecutable que permita probar hoy el flujo token → principal → membresías,
   con un usuario admin precreado.
2. Una frontera de escritura admin en PostgreSQL que no dependa de un GUC falsificable.
3. Pruebas de integración contra PostgreSQL real, ejecutables localmente y activables en CI.
4. Un hueco explícito y documentado para el tenant Auth0 real y su usuario admin.

## Componentes

### 1. `backend/internal/devoidc` + `backend/cmd/devoidc`

Emisor OIDC de desarrollo, fuera del camino de producción:

- Clave RSA efímera en memoria; JWKS servido en `/keys`.
- Discovery en `/.well-known/openid-configuration` con `issuer`, `jwks_uri`, `id_token_signing_alg_values_supported: ["RS256"]`.
- `POST /oauth/token`: JSON con `client_id`, `client_secret`, `username`, `password`, `audience`;
  devuelve `access_token` (JWT RS256 con `iss`, `sub`, `aud`, `exp`, `iat`) y `token_type`.
- Usuarios de desarrollo definidos por configuración, incluido el admin (subject `dev|admin`).
- Guardarraíles: escucha solo en loopback; el binario se registra como herramienta de desarrollo y
  no se importa desde `cmd/api`; credenciales de desarrollo sintéticas, nunca valores reales.

La configuración de Auth0 acepta `http://127.0.0.1` como única excepción a la
regla HTTPS, porque el emisor local es HTTP. Cualquier otro host sigue requiriendo HTTPS.
> Actualización 2026-11-18: la excepción se restringió a una IP literal de loopback. `http://localhost`
> se rechaza, porque depende de DNS y del archivo `hosts` y no prueba que el tráfico se quede en la
> máquina. Ver `docs/architecture/backend-next-phase.md`.

### 2. Frontera de escritura admin en PostgreSQL — migración `000003`

Roles (creados por `backend/deploy/postgres/bootstrap.sql`, ejecutado como superusuario antes de migrar):

| Rol | Uso | Atributos |
| --- | --- | --- |
| `energyhub_runtime` | credencial `DATABASE_URL` de la API | `LOGIN`, solo `SELECT` sobre las tablas |
| `energyhub_membership_admin` | grupo de escritura admin | `NOLOGIN` |
| `energyhub_admin` | credencial `ADMIN_DATABASE_URL` | `LOGIN`, miembro del grupo admin |
| `energyhub_owner` | dueño de funciones y tablas | `NOLOGIN`, `BYPASSRLS` |

La migración `000003` hace lo siguiente:

- Crea `app.grant_membership`, `app.revoke_membership` y `app.create_organization` como
  `SECURITY DEFINER`, con `SET search_path = ''` y referencias cualificadas.
- Revoca `EXECUTE` a `PUBLIC` y lo concede solo a `energyhub_membership_admin`.
- Retira las políticas de escritura basadas en `app.is_system_admin`.
- Concede `SELECT` al rol runtime (RLS sigue aplicando) y `INSERT/UPDATE/DELETE/SELECT` a
  `energyhub_owner` (necesario para que las funciones escriban).
- La lectura sigue aislada por `app.current_subject`, transaction-local, fijado por el servidor a
  partir del subject verificado.

El GUC `app.is_system_admin` desaparece: la frontera pasa a ser la credencial de BD. `SYSTEM_ADMIN_SUBJECT`
se mantiene como segunda barrera en la capa de aplicación (quién puede usar la credencial admin).

### 3. Código Go

- `MembershipWriter` deja de insertar directamente: llama a `app.grant_membership` / `app.revoke_membership`
  usando el pool admin, tras comprobar `SYSTEM_ADMIN_SUBJECT`.
- `MembershipReader` conserva la transacción con `app.current_subject` y el pool runtime.
- Configuración: `DATABASE_URL` (runtime) y `ADMIN_DATABASE_URL` (admin). Si falta la admin, el
  servicio de membresías no se compone y no se registra ninguna ruta administrativa.
- `cmd/api` compone `/health` siempre; la ruta protegida `/v1/organizations` (solo membresías del
  propio sujeto, sin datos de dispositivos) solo se registra si hay issuer, audience y `DATABASE_URL`.

### 4. Pruebas

- Unitarias del emisor: discovery, JWKS, emisión, credenciales incorrectas, audiencia incorrecta.
- Integración del verificador Auth0 contra el emisor del paquete (reemplaza el `httptest` duplicado).
- Contrato SQL de la migración `000003` (roles, funciones, `REVOKE` a `PUBLIC`, políticas).
- Integración real con PostgreSQL, activada con `TEST_DATABASE_URL`: aislamiento de lectura,
  denegación de escritura directa al rol runtime, denegación de la función admin al rol runtime,
  escritura admin permitida vía función.
- HTTP: 401 sin token, 200 con token, lista de organizaciones limitada al sujeto.

## Límites declarados

- El emisor es de desarrollo: no implementa Authorization Code + PKCE, no valida contra Auth0 y no
  debe usarse en despliegues reales.
- `app.current_subject` sigue siendo un GUC: el aislamiento de lectura confía en la credencial de BD
  de la API. Se documenta como frontera blanda; el aislamiento entre tenants del cliente HTTP no
  depende de ella (el subject lo fija el servidor a partir del token verificado).
- La base de datos local de desarrollo se crea y destruye con scripts locales; no hay datos reales.
- Cuando exista el tenant Auth0: crear API, copiar `AUTH0_ISSUER`/`AUTH0_AUDIENCE`, crear el usuario
  admin, copiar su `sub` a `SYSTEM_ADMIN_SUBJECT` y ejecutar la misma prueba de integración.

## Estado de implementación (2026-11-17)

Commits que materializan el diseño:

- `1af7f21`: emisor OIDC local (`internal/devoidc`, `cmd/devoidc`) y excepción de loopback en la
  validación del issuer.
- `568266f`: frontera de escritura admin por rol (`migrations/000003_secure_admin_writes.up.sql`,
  `deploy/postgres/bootstrap.sql`) y writers que llaman a las funciones `app.*`.
- `5172f27`: ruta protegida `GET /v1/organizations`, CLI de administración `cmd/membership`,
  `MIGRATION_DATABASE_URL` en `cmd/migrate` y `scripts/dev-stack.sh`.
- `61fc573`, `83e69b2`, `ca13558`: índice de evidencia, ignorado del hook `graphify` y cobertura
  adicional (`storage/postgres` 100%, `httpapi` 93.9%).

Verificado: `go test ./...`, `go vet ./...`, `go build ./...`, `go mod verify` y `go test -race ./...`
en verde; integración real con 9 subcasos contra PostgreSQL 17.11; flujo de extremo a extremo
registrado en `docs/evidence/backend-contract-first-2026-11-16/local-stack-e2e.md`.

Pendiente:

1. Tenant Auth0 real: crear la API, copiar `AUTH0_ISSUER`, `AUTH0_AUDIENCE` y `SYSTEM_ADMIN_SUBJECT`
   y repetir la misma prueba. Solo cambia la configuración; el verificador no cambia.
2. Verificar RLS contra la instancia PostgreSQL de despliegue (la prueba local ya pasa) y aplicar la
   política de rotación de credenciales documentada en `docs/architecture/backend-next-phase.md`.
3. Actualizar el plan de Notion cuando el conector vuelva a estar disponible.

## Estado de implementación (2026-11-18)

Seguimiento del diseño anterior, con dos commits adicionales:

- `2655a85`: migración `000004_harden_admin_role_boundary`, que traslada la propiedad del esquema `app`
  y de las tablas a `energyhub_owner`, retira el `SELECT` global de la credencial admin, exige a la vez
  el grupo `energyhub_membership_admin` y el rol `energyhub_admin_capability` para escribir, audita las
  escrituras en `app.admin_audit_log`, hace explícito el conflicto de creación duplicada y endurece
  `cmd/migrate` y `cmd/api` (DSN de migración obligatoria, rechazo del rol runtime, verificación del
  rol de la API al arrancar).
- Endurecimiento de la superficie HTTP y del stack local: límites de tiempo y de cabecera en los
  servidores, `no-store` y `Vary: Authorization` en la ruta protegida, timeout del cliente OIDC,
  issuer de desarrollo restringido a IP literal de loopback con cuerpo limitado y sin secretos en el
  log, y `dev-stack.sh` sin interpolación SQL ni credencial admin en el proceso API.

- Revisión de código y de seguridad posterior, que añadió la migración `000005`: `000004` no detectaba
  `TRUNCATE`, que el RLS no cubre, y no reafirmaba la propiedad de las funciones administrativas sobre
  `energyhub_owner`. El bootstrap pasó a resolver la pertenencia transitiva de roles, de modo que una
  cadena de `GRANT` o una credencial administrativa compartida hacen fallar el despliegue en vez de
  abrir una vía silenciosa.

- Última ronda, con tres hallazgos propios y una brecha de proceso:

  - La suite de integración liberaba los privilegios del rol temporal solo después de eliminar la base
    de prueba, así que el rol con `TRUNCATE` no se podía borrar y el cluster acumulaba roles
    `energyhub_it_truncate_*`. Ahora los privilegios se liberan antes de borrar el rol y el cluster
    queda limpio, comprobado con `pg_roles` y `pg_database`.
  - `cmd/api`, `cmd/membership` y `cmd/migrate` quitan la contraseña del DSN de sus errores de conexión:
    el driver de PostgreSQL la incluye sin filtrar en el error de una URL mal formada y ese error
    llegaba al log de arranque.
  - `dev-stack.sh` exportaba el DSN del runtime como argumento de `env`, así que era visible con `ps`.
    Ahora el API arranca en un subshell que elimina las credenciales administrativas del entorno heredado
    y exporta el DSN del runtime.
  - El job de backend de CI levanta un PostgreSQL 17 como servicio y ejecuta la suite `TestIntegration`,
    de modo que el aislamiento y la frontera administrativa se comprueban en cada ejecución y no solo en
    la máquina del desarrollador.
  - La reversión de `000003` archivaba mal la evidencia: eliminaba la traza de auditoría sin archivarla
    cuando se usaba sin la reversión de `000004`. En esta ronda archivaba en
    `public.admin_audit_log_archive_000003` antes de eliminar el esquema `app`, y el `.down.sql` dejaba al
    rol runtime con lectura. La corrección no toca el `.up.sql` de `000003`: los `.down.sql` son
    referencias manuales y no se ejecutan automáticamente, así que no cambian el estado de ninguna base
    ya aplicada. Este enfoque quedó superado el 2026-11-19: las reversiones son fail-closed y no archivan
    nada (ver más abajo).

Verificado: `go test ./...`, `go vet ./...`, `go build ./...`, `go mod verify`, `go test -race ./...` en
verde; integración real contra PostgreSQL 17.11 con el aislamiento, la auditoría, el conflicto de
creación duplicada, el rollback de `000004`, `000003` y `000005`, el archivado de la auditoría en las
reversiones, la revocación de `TRUNCATE`, la propiedad de los objetos y de las funciones, el fallo
cerrado del bootstrap ante una pertenencia transitiva y la migración con un rol no superusuario.
Toolchain fijada en Go 1.27.4.

Pendiente: la prueba contra el tenant Auth0 real, el rate limiting por IP o subject, la decisión sobre
roles de PostgreSQL por tenant para eliminar la falsificación de `app.current_subject` y la
actualización del plan de Notion.

## Estado de implementación (2026-11-19)

Ronda de revisión de seguridad que cerró los hallazgos del informe de seguridad (veredicto previo:
DO NOT SHIP):

- **Bypass de la comprobación del rol runtime** (alto). El verificador comparaba `current_user` con el
  rol runtime esperado, pero `current_user` puede cambiarse con `SET ROLE` o con una opción de arranque
  (`options=-c role=...`): una sesión autenticada como superusuario podía pasar la comprobación fingiendo
  ser el rol de solo lectura y recuperar después sus privilegios. El verificador ahora se apoya en
  `session_user` (la identidad que PostgreSQL autenticó), exige `session_user = current_user` y rechaza
  que el rol pueda asumir un rol privilegiado (`energyhub_owner`, `energyhub_admin`,
  `energyhub_membership_admin`, `energyhub_admin_capability`) o un rol con `BYPASSRLS`, recorriendo todo
  el grafo de pertenencias, de modo que una pertenencia con `INHERIT FALSE, SET TRUE` tampoco sirve para
  eludirla. Cubierto por dos pruebas de integración reales (`def2f58`).

- **Higiene de credenciales del stack local** (medio). `dev-stack.sh` exportaba las contraseñas de base de
  datos al proceso global y el API heredaba `DATABASE_URL` y `ADMIN_DATABASE_URL`; además, el script
  imprimía los DSN con contraseña en la salida estándar, que acaba en logs de terminal, en ficheros
  redirigidos y en logs de CI. Ahora las contraseñas no se exportan (solo la llamada a `psql` que las
  necesita las recibe), el API y el emisor heredan un entorno limpio, y el entorno con los DSN se escribe en
  `.local/dev-stack.env` con modo `0600`; el script solo imprime la ruta (`30e53cc`).

- **Lectura de la traza de auditoría por la credencial administrativa** (medio). `000004` había concedido
  `SELECT` sobre `app.admin_audit_log` a `energyhub_membership_admin`, así que la credencial administrativa
  podía leer toda la traza del sistema. La migración nueva `000006_close_admin_audit_read` revoca ese
  privilegio y el de `energyhub_admin`, revoca `CREATE` sobre el esquema `public` a `PUBLIC` (revocar a los
  roles de la aplicación no anula la concesión a `PUBLIC`) y `USAGE` sobre el esquema `app` a `PUBLIC`. El
  bootstrap hace lo mismo y comprueba que ningún rol de la aplicación puede crear objetos en `public`
  (`54883be`).

- **Reversiones que reabrían la frontera** (medio). Los `.down.sql` de `000003` y `000004` restauraban las
  políticas de escritura falsificables (`app.is_system_admin`), devolvían al grupo administrativo la lectura
  de las tablas de todos los tenants y eliminaban la traza de auditoría. Ahora todas las reversiones son
  forward-only: verifican que el estado endurecido sigue en pie y fallan cerrado cuando no lo está. El
  archivado de la auditoría desaparece con ello, porque sobrescribir un archivo de archivo previo destruía
  la evidencia de un periodo anterior (`54883be`).

- `000006` es una migración nueva y no una edición de `000004`: una migración ya aplicada no se edita. Los
  `.down.sql` son referencias manuales que nunca se ejecutan automáticamente, así que corregirlos no cambia
  el estado de ninguna base ya aplicada.

Verificado: `go test ./...`, `go build ./...`, `gofmt` sin diferencias y la integración real contra
PostgreSQL 17.11 en verde, incluida la reversión fail-closed de las cuatro migraciones, la denegación de
lectura de la auditoría a la credencial administrativa y la denegación de escritura directa al rol runtime.
Comprobado además que el stack local no filtra contraseñas por la salida estándar ni por la lista de
procesos.

Pendiente: reejecutar la revisión de código y la de seguridad sobre el rango completo antes de publicar,
la prueba contra el tenant Auth0 real, el rate limiting por IP o subject, la decisión sobre roles de
PostgreSQL por tenant para eliminar la falsificación de `app.current_subject` y la actualización del plan de
Notion.
