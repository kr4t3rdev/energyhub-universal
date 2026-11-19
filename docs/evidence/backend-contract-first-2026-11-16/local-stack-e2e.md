# Evidencia — Stack local y flujo de extremo a extremo

Fecha: 2026-11-17 (actualizado 2026-11-18). PostgreSQL 17.11 (Homebrew), emisor OIDC de desarrollo, API local.

## Qué se probó

1. El emisor de desarrollo emite tokens RS256 con `iss`, `sub`, `aud` y `exp`.
2. `cmd/api` verifica el token contra el emisor y resuelve las organizaciones del
   sujeto verificado en PostgreSQL.
3. `cmd/membership` crea organizaciones y concede o revoca membresías usando la
   credencial administrativa.
4. Un usuario sin membresía no ve las organizaciones de otro usuario.
5. La credencial administrativa no puede leer ni escribir directamente las tablas
   de tenants: solo puede ejecutar las funciones `app.*`.
6. Las escrituras administrativas quedan registradas en `app.admin_audit_log`.
7. La API se niega a arrancar con una credencial de base de datos superusuario o
   con escritura, y con el emisor de desarrollo si no escucha en loopback.

## Comandos

```sh
cd backend
PGPORT=55432 PGUSER=postgres bash scripts/dev-stack.sh
```

El script crea los roles, aplica las migraciones, arranca el emisor local e
imprime las variables de entorno para la API. La API del stack local escucha en
`127.0.0.1:8080` y no recibe la credencial administrativa.

## Resultados observados

Token del emisor para `dev|user-1`:

```
curl -s -X POST http://127.0.0.1:8089/oauth/token \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"energyhub-dev","client_secret":"dev-secret","username":"user1","password":"user1-dev","audience":"https://api.energyhub.local"}'
```

Operaciones administrativas:

```
create-organization completed for organization "org-1"
grant completed for organization "org-1"
revoke completed for organization "org-1"
```

Crear de nuevo la misma organización devuelve un conflicto explícito en lugar de
renombrarla en silencio.

Ruta protegida:

| Petición | Resultado |
| --- | --- |
| `GET /health` | `200` con `{"status":"ok"}` |
| `GET /v1/organizations` sin token | `401` con sobre `unauthenticated` |
| `GET /v1/organizations` con token manipulado | `401` |
| `GET /v1/organizations` con token de `dev|user-1` | `200` con `org-1` y rol `viewer` |
| `GET /v1/organizations` con token de `dev|admin` sin membresía | `200` con lista vacía |
| `GET /v1/organizations` con token de `dev|admin` tras conceder `org-2` | `200` con `org-2` y rol `owner` |

Las respuestas de la ruta protegida llevan `Cache-Control: no-store` y
`Vary: Authorization`, también en los sobres de error, incluido el `401`: las
cabeceras se fijan antes de la autenticación.

Aislamiento entre tenants: tras conceder `org-2` solo a `dev|admin`, la petición
de `dev|user-1` siguió devolviendo únicamente `org-1`.

Frontera administrativa, con la credencial `energyhub_admin`:

| Operación | Resultado |
| --- | --- |
| `SELECT` sobre `organizations` | `permission denied` |
| `INSERT` sobre `organization_memberships` | `permission denied` |
| `SELECT app.create_organization(...)` | `permission denied` sin el rol de capacidad |

La traza de auditoría registra el actor, la operación y el objetivo de cada
escritura administrativa.

Arranque de la API:

| Configuración | Resultado |
| --- | --- |
| Credencial de base de datos superusuario | el proceso no arranca |
| Emisor de desarrollo con `LISTEN_ADDR=0.0.0.0:8080` | el proceso no arranca |
| Emisor de desarrollo con `LISTEN_ADDR=127.0.0.1:8080` | arranca y expone la ruta protegida |

## Pruebas automatizadas

- `internal/platform/httpapi`: 401 sin token, 401 con token inválido, 200 con las
  organizaciones del sujeto, 500 genérico si el reader falla y sin filtrar detalles
  internos; las rutas protegidas no se registran sin verificador o reader; las
  respuestas de éxito y de error no se pueden cachear.
- `internal/authn`: el middleware admite un respondedor de errores para mantener
  el sobre JSON y sigue sin exponer detalles del verificador; el issuer sobre
  HTTP plano solo se acepta con una IP literal de loopback.
- `cmd/api`: la configuración de autenticación y base de datos se exige en bloque;
  sin configuración solo se expone `/health`; el emisor de desarrollo exige una
  dirección de escucha loopback.
- `cmd/devoidc`: la dirección debe ser una IP literal de loopback y el arranque no
  imprime el secreto del cliente ni las contraseñas.
- `cmd/membership`: análisis de comandos, validación de rol y requisitos de
  `ADMIN_DATABASE_URL` y `SYSTEM_ADMIN_SUBJECT`.
- `cmd/migrate`: `MIGRATION_DATABASE_URL` es obligatoria y el rol `energyhub_runtime`
  se rechaza como migrador, tanto por el userinfo de la URL como por el rol
  effective de la sesión.
- `internal/storage/postgres`: prueba de integración real sobre el aislamiento y
  la frontera administrativa, la auditoría, el conflicto de creación duplicada, el
  rollback de `000004` y `000003` (con el archivo de auditoría archivado en vez de
  borrado), el rollback de `000005`, la revocación de `TRUNCATE`, la propiedad de
  los objetos y de las funciones administrativas en `energyhub_owner`, el fallo
  cerrado del bootstrap ante una pertenencia transitiva o una credencial
  administrativa compartida, y la migración ejecutada por un rol no superusuario.
  La suite también libera los privilegios del rol temporal antes de eliminarlo:
  después de la ejecución, `pg_roles` y `pg_database` no muestran ningún rol ni
  base `energyhub_it_%`.
- `internal/storage/postgres` (redaction): la contraseña del DSN no aparece en los
  errores de conexión de `cmd/api`, `cmd/membership` ni `cmd/migrate`, que la
  quitan antes de mostrarlos.

Higiene de credenciales en el proceso:

| Comprobación | Resultado |
| --- | --- |
| DSN del runtime en la lista de procesos | no aparece (`ps -Ao args` no lo encuentra) |
| `ADMIN_DATABASE_URL` en el entorno del proceso API | no está |
| `MIGRATION_DATABASE_URL` en el entorno del proceso API | no está |

La suite de integración corre también en CI: el job de backend levanta un
PostgreSQL 17 como servicio y ejecuta `TestIntegration`, así que el aislamiento y
la frontera administrativa se comprueban en cada ejecución.

## Límites explícitos

- El emisor es de desarrollo: no es Auth0, no implementa Authorization Code + PKCE
  y escucha solo en loopback.
- La API expone únicamente `/v1/organizations` (membresías del propio sujeto). No
  hay endpoints de datos de dispositivos, telemetría ni hardware.
- La administración es por CLI, no por HTTP: no existe todavía una superficie HTTP
  que conceda membresías.
- Las credenciales del stack local son de desarrollo y no sirven fuera del entorno
  local. El script las imprime en la salida estándar, así que esa salida no debe
  copiarse a un registro compartido.
- `app.current_subject` es un GUC de sesión falsificable: quien tenga la credencial
  runtime puede fijar un subject arbitrario. La credencial de base de datos es la
  frontera de confianza de la lectura y la limitación está fijada por la prueba
  `TestIntegrationDocumentsForgeableTenantContext`.
- No hay rate limiting por IP ni por subject: sigue pendiente antes de exponer el
  servicio a datos reales.
