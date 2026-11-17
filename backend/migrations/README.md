# backend/migrations

Migraciones SQL forward-only, embebidas por `migrations.Files` para que el comando no dependa del directorio de trabajo.

- Nombre: `NNNNNN_snake_case.up.sql`; el `.down.sql` correspondiente queda como referencia para una reversión manual revisada.
- No editar una migración ya aplicada: crear una nueva.
- La API no ejecuta migraciones. Solo se aplican de forma explícita:

```sh
DATABASE_URL='postgres://...' go run ./cmd/migrate
```

El runner crea `schema_migrations`, aplica cada versión pendiente y registra esa versión en la misma transacción. Se niega a continuar si la base tiene una versión que el binario no conoce. Los archivos `.down.sql` no se ejecutan automáticamente.

Ejecutar como un único job de despliegue: no hay coordinación entre runners concurrentes, pero la clave primaria de `schema_migrations` hace que un duplicado falle en lugar de aplicar dos veces la misma versión.

## RLS

`000002_enable_tenant_rls` habilita y fuerza RLS en `organizations` y `organization_memberships`. Las lecturas deben fijar `app.current_subject` en la transacción; el lector de `internal/storage/postgres` lo hace automáticamente. Sin esa variable, las políticas deniegan el acceso.

## Roles y frontera de escritura administrativa

`000003_secure_admin_writes` sustituye la escritura administrativa basada en `app.is_system_admin` por una frontera de rol de PostgreSQL. Un GUC personalizado no puede protegerse: cualquier rol puede ejecutar `SET app.is_system_admin = 'on'`, así que con la credencial de la aplicación era posible escalarse a escritura admin.

Antes de aplicar las migraciones hay que crear los roles del cluster (una vez, con un superusuario):

```sh
psql "$DATABASE_ADMIN_URL" -v ON_ERROR_STOP=1 \
  -f backend/deploy/postgres/bootstrap.sql
```

El bootstrap solo crea roles y permisos: no asigna contraseñas, para que ninguna credencial viva en el repositorio. El operador las fija desde su gestor de secretos, por ejemplo:

```sh
psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_runtime PASSWORD '$RUNTIME_PASSWORD'"
psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_admin PASSWORD '$ADMIN_PASSWORD'"
```

| Rol | Uso |
| --- | --- |
| `energyhub_runtime` | credencial `DATABASE_URL` de la API: solo `SELECT`, con RLS activo |
| `energyhub_membership_admin` | grupo `NOLOGIN` que habilita las escrituras administrativas |
| `energyhub_admin` | credencial `ADMIN_DATABASE_URL`: miembro del grupo admin |
| `energyhub_owner` | `NOLOGIN BYPASSRLS`, dueño de las funciones `app.*` |

Las escrituras pasan por `app.create_organization`, `app.grant_membership` y `app.revoke_membership`: funciones `SECURITY DEFINER` con `search_path` vacío, `EXECUTE` revocado a `PUBLIC` y concedido solo al grupo admin. Cada función vuelve a comprobar `pg_has_role(session_user, 'energyhub_membership_admin', 'member')`, así que un `GRANT` accidental no abre la puerta. La API escribe con la credencial admin y nunca directamente sobre las tablas.

Las migraciones se aplican con un rol que pueda cambiar la propiedad de las funciones a `energyhub_owner`: un superusuario o un rol con esa pertenencia.

El `.down.sql` de `000003` restaura el conjunto de políticas de `000002`, incluida la frontera más débil basada en `app.is_system_admin`. Solo usarlo para volver a `000002` en un entorno controlado.
