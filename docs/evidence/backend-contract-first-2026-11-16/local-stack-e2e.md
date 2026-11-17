# Evidencia — Stack local y flujo de extremo a extremo

Fecha: 2026-11-17. PostgreSQL 17.11 (Homebrew), emisor OIDC de desarrollo, API local.

## Qué se probó

1. El emisor de desarrollo emite tokens RS256 con `iss`, `sub`, `aud` y `exp`.
2. `cmd/api` verifica el token contra el emisor y resuelve las organizaciones del
   sujeto verificado en PostgreSQL.
3. `cmd/membership` crea organizaciones y concede o revoca membresías usando la
   credencial administrativa.
4. Un usuario sin membresía no ve las organizaciones de otro usuario.

## Comandos

```sh
cd backend
PGPORT=55432 PGUSER=postgres bash scripts/dev-stack.sh
```

El script crea los roles, aplica las migraciones, arranca el emisor local e
imprime las variables de entorno para la API.

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

Ruta protegida:

| Petición | Resultado |
| --- | --- |
| `GET /health` | `200` con `{"status":"ok"}` |
| `GET /v1/organizations` sin token | `401` con sobre `unauthenticated` |
| `GET /v1/organizations` con token manipulado | `401` |
| `GET /v1/organizations` con token de `dev|user-1` | `200` con `org-1` y rol `viewer` |
| `GET /v1/organizations` con token de `dev|admin` sin membresía | `200` con lista vacía |
| `GET /v1/organizations` con token de `dev|admin` tras conceder `org-2` | `200` con `org-2` y rol `owner` |

Aislamiento entre tenants: tras conceder `org-2` solo a `dev|admin`, la petición
de `dev|user-1` siguió devolviendo únicamente `org-1`.

## Pruebas automatizadas

- `internal/platform/httpapi`: 401 sin token, 401 con token inválido, 200 con las
  organizaciones del sujeto, 500 genérico si el reader falla y sin filtrar detalles
  internos; las rutas protegidas no se registran sin verificador o reader.
- `internal/authn`: el middleware admite un respondedor de errores para mantener
  el sobre JSON y sigue sin exponer detalles del verificador.
- `cmd/api`: la configuración de autenticación y base de datos se exige en bloque;
  sin configuración solo se expone `/health`.
- `cmd/membership`: análisis de comandos, validación de rol y requisitos de
  `ADMIN_DATABASE_URL` y `SYSTEM_ADMIN_SUBJECT`.
- `cmd/migrate`: `MIGRATION_DATABASE_URL` tiene prioridad sobre `DATABASE_URL`.
- `internal/storage/postgres`: prueba de integración real con 9 subcasos sobre el
  aislamiento y la frontera administrativa.

## Límites explícitos

- El emisor es de desarrollo: no es Auth0, no implementa Authorization Code + PKCE
  y escucha solo en loopback.
- La API expone únicamente `/v1/organizations` (membresías del propio sujeto). No
  hay endpoints de datos de dispositivos, telemetría ni hardware.
- La administración es por CLI, no por HTTP: no existe todavía una superficie HTTP
  que conceda membresías.
- Las credenciales del stack local son de desarrollo y no sirven fuera del entorno
  local.
