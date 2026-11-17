# Diseño — Emisor OIDC local y frontera de escritura admin

Fecha: 2026-11-17
Estado: aprobado por el usuario (frontera de admin: doble credencial + `SECURITY DEFINER`)

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

La configuración de Auth0 acepta `http://127.0.0.1` / `http://localhost` como única excepción a la
regla HTTPS, porque el emisor local es HTTP. Cualquier otro host sigue requiriendo HTTPS.

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
