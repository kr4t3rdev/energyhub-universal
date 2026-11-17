# Emisor OIDC de desarrollo

`cmd/devoidc` es un emisor OIDC **solo para desarrollo**. Existe porque todavía no hay un tenant
Auth0 y hace falta probar el flujo token → principal → membresías con un usuario admin real.

No es un emulador de Auth0: implementa solo el subconjunto que usa la API (discovery, JWKS y un
endpoint de token) y **no debe desplegarse** como proveedor de identidad real.

## Guardarraíles

- Escucha solo en loopback (`127.0.0.1` por defecto). `DEV_OIDC_ADDR` con una dirección no loopback
  hace fallar el arranque.
- Las identidades son sintéticas (`dev|admin`, `dev|user-1`) y las credenciales son de desarrollo.
- La clave RSA de firma es efímera: se genera en memoria al arrancar, así que reiniciar invalida
  los tokens emitidos.
- El paquete `internal/devoidc` no lo importa `cmd/api`.

## Uso

```bash
cd backend
go run ./cmd/devoidc
```

La salida imprime los valoresexactos que necesita la API:

```
AUTH0_ISSUER=http://127.0.0.1:8089
AUTH0_AUDIENCE=https://api.energyhub.local
SYSTEM_ADMIN_SUBJECT=dev|admin
```

Pedir un token:

```bash
curl -s -X POST http://127.0.0.1:8089/oauth/token \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"energyhub-dev","client_secret":"dev-secret","username":"admin","password":"admin-dev","audience":"https://api.energyhub.local"}'
```

La respuesta contiene `access_token` (JWT RS256 con `iss`, `sub`, `aud`, `exp`) y `token_type`.

## Variables

| Variable | Por defecto | Uso |
| --- | --- | --- |
| `DEV_OIDC_ADDR` | `127.0.0.1:8089` | Dirección de escucha (loopback) |
| `DEV_OIDC_AUDIENCE` | `https://api.energyhub.local` | Audiencia de los tokens emitidos |
| `DEV_OIDC_CLIENT_ID` | `energyhub-dev` | Client id del endpoint de token |
| `DEV_OIDC_CLIENT_SECRET` | `dev-secret` | Client secret del endpoint de token |
| `DEV_OIDC_ADMIN_SUBJECT` | `dev|admin` | Subject del usuario admin |
| `DEV_OIDC_ADMIN_USERNAME` | `admin` | Usuario admin |
| `DEV_OIDC_ADMIN_PASSWORD` | `admin-dev` | Contraseña del admin |
| `DEV_OIDC_USER_SUBJECT` | `dev|user-1` | Subject del usuario normal |
| `DEV_OIDC_USER_USERNAME` | `user1` | Usuario normal |
| `DEV_OIDC_USER_PASSWORD` | `user1-dev` | Contraseña del usuario normal |

## Cuando exista el tenant Auth0

1. Crear una API en Auth0 y copiar su identifier a `AUTH0_AUDIENCE`.
2. Copiar el dominio del tenant a `AUTH0_ISSUER` (formato `https://<tenant>.<region>.auth0.com/`).
3. Crear el usuario admin y copiar su `sub` a `SYSTEM_ADMIN_SUBJECT`.
4. Arrancar la API con esos valores: el verificador no cambia, solo la configuración.

La configuración de Auth0 acepta `http` únicamente para emisores loopback; cualquier otro host
sigue exigiendo HTTPS.
