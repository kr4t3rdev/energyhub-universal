# backend/migrations

Migraciones SQL forward-only, embebidas por `migrations.Files` para que el comando no dependa del directorio de trabajo.

- Nombre: `NNNNNN_snake_case.up.sql`; el `.down.sql` correspondiente queda como referencia para una reversión manual revisada.
- No editar una migración ya aplicada: crear una nueva.
- La API no ejecuta migraciones. Solo se aplican de forma explícita:

```sh
MIGRATION_DATABASE_URL='postgres://...' go run ./cmd/migrate
```

`MIGRATION_DATABASE_URL` es obligatoria y no tiene alternativa: la credencial de la API (`DATABASE_URL`) no puede aplicar migraciones, porque quedaría como propietaria de las tablas. El comando rechaza la conexión si el usuario es `energyhub_runtime`.

El runner crea `schema_migrations`, aplica cada versión pendiente y registra esa versión en la misma transacción. Se niega a continuar si la base tiene una versión que el binario no conoce. Los archivos `.down.sql` no se ejecutan automáticamente.

Ejecutar como un único job de despliegue: no hay coordinación entre runners concurrentes, pero la clave primaria de `schema_migrations` hace que un duplicado falle en lugar de aplicar dos veces la misma versión.

## RLS

`000002_enable_tenant_rls` habilita y fuerza RLS en `organizations` y `organization_memberships`. Las lecturas deben fijar `app.current_subject` en la transacción; el lector de `internal/storage/postgres` lo hace automáticamente. Sin esa variable, las políticas deniegan el acceso.

`app.current_subject` es un GUC y un GUC no se puede proteger en PostgreSQL: la credencial de la API puede fijar otro valor por su cuenta. El aislamiento entre tenants del cliente HTTP no depende de él (el subject lo fija el servidor a partir del token verificado), pero una credencial de base de datos filtrada sí puede leer las membresías de otro sujeto. La prueba `TestIntegrationDocumentsForgeableTenantContext` deja esa limitación escrita y falla si el comportamiento cambia.

## Roles y frontera de escritura administrativa

`000003_secure_admin_writes` sustituye la escritura administrativa basada en `app.is_system_admin` por una frontera de rol de PostgreSQL. Un GUC personalizado no puede protegerse: cualquier rol puede ejecutar `SET app.is_system_admin = 'on'`, así que con la credencial de la aplicación era posible escalarse a escritura admin.

`000004_harden_admin_role_boundary` cierra los huecos que dejó `000003`:

- La propiedad del esquema `app` y de las tablas pasa a `energyhub_owner`, un rol `NOLOGIN BYPASSRLS`. Antes, el rol que aplicó las migraciones conservaba la propiedad y podía desactivar RLS, borrar políticas o concederse escrituras.
- Se revocan todos los privilegios previos y se concede el mínimo: la API solo lee, la credencial administrativa solo ejecuta funciones.
- Las políticas de lectura ya no conceden acceso total al grupo administrativo.
- Las funciones comprueban `energyhub_admin_capability`, un grupo distinto del que tiene el `EXECUTE`, así que conceder uno de los dos por error no abre la puerta.
- Cada escritura administrativa deja una traza en `app.admin_audit_log` con el actor (`session_user`), la operación y el objetivo.
- Crear una organización que ya existe es un conflicto explícito, no un renombrado silencioso.

`000005_revoke_leftover_protected_table_privileges` cierra el hueco que quedó en `000004`:

- La detección de privilegios sobrantes de `000004` no incluía `TRUNCATE`, y el RLS no se aplica a `TRUNCATE`: un rol con solo ese privilegio podía borrar las tablas de tenants. `000005` revoca `INSERT`, `UPDATE`, `DELETE` y `TRUNCATE` sobre `organizations`, `organization_memberships` y `app.admin_audit_log` a todos los roles que no heredan `energyhub_owner`, y falla cerrado si queda alguno.
- La propiedad de las funciones administrativas se vuelve a afirmar sobre `energyhub_owner`, con una comprobación posterior. `CREATE OR REPLACE` conserva el propietario anterior, así que sin esta afirmación una restauración de la base podía dejar las funciones en manos del rol que las instaló, que podría reemplazar su cuerpo y quitarse la comprobación de capacidad.
- `000005` es una migración nueva y no una edición de `000004`, porque una migración ya aplicada no se edita.

Antes de aplicar las migraciones hay que crear y normalizar los roles del cluster (una vez, con un superusuario y contra la base de datos de la aplicación):

```sh
psql "$DATABASE_ADMIN_URL" -v ON_ERROR_STOP=1 \
  -f backend/deploy/postgres/bootstrap.sql
```

El bootstrap normaliza los atributos de los roles en cada ejecución, revoca las pertenencias que no corresponden, crea el esquema `app` con `energyhub_owner` como propietario y falla si un rol inesperado pertenece a los grupos administrativos. No asigna contraseñas, para que ninguna credencial viva en el repositorio. El operador las fija desde su gestor de secretos, por ejemplo:

```sh
psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_runtime PASSWORD '$RUNTIME_PASSWORD'"
psql "$DATABASE_ADMIN_URL" -c "ALTER ROLE energyhub_admin PASSWORD '$ADMIN_PASSWORD'"
```

Las comprobaciones del bootstrap resuelven la pertenencia completa, no solo la directa: un rol es miembro de otro de forma transitiva, y un superusuario es miembro de todos. Por eso una cadena como `GRANT energyhub_owner TO helper; GRANT helper TO energyhub_runtime` no aparece en `pg_auth_members` pero sí concede a la credencial de la API los derechos de propietario, y por eso el bootstrap falla cerrado en ese caso. El mismo control cubre `energyhub_admin`: compartir la credencial administrativa con otro rol de login crearía una segunda credencial de escritura sin que nadie lo anunciara.

| Rol | Uso |
| --- | --- |
| `energyhub_runtime` | credencial `DATABASE_URL` de la API: solo `SELECT`, con RLS activo |
| `energyhub_membership_admin` | grupo `NOLOGIN` que habilita las escrituras administrativas |
| `energyhub_admin_capability` | grupo `NOLOGIN` que las funciones comprueban |
| `energyhub_admin` | credencial `ADMIN_DATABASE_URL`: miembro de los dos grupos |
| `energyhub_owner` | `NOLOGIN BYPASSRLS`, dueño del esquema, de las tablas y de las funciones |

Las escrituras pasan por `app.create_organization`, `app.grant_membership` y `app.revoke_membership`: funciones `SECURITY DEFINER` con `search_path` vacío, `EXECUTE` revocado a `PUBLIC` y concedido solo al grupo administrativo. El CLI de administración escribe con la credencial admin; la API solo lee con `DATABASE_URL` y nunca escribe.

Las migraciones se aplican con un rol que pueda cambiar la propiedad de los objetos a `energyhub_owner`: un superusuario o un rol con esa pertenencia. Ese rol hereda `BYPASSRLS` y los privilegios de escritura, así que es una credencial privilegiada del job de despliegue: nunca debe ser la `DATABASE_URL` de la API.

El `.down.sql` de `000004` devuelve el estado de `000003` y el de `000003` devuelve el de `000002`, incluida la frontera más débil basada en `app.is_system_admin`. Son reversiones manuales: se ejecutan con una conexión privilegiada y solo para volver a un estado anterior en un entorno controlado.

Los `.down.sql` no se ejecutan automáticamente, así que corregirlos no cambia el estado de ninguna base ya aplicada: la regla de no editar una migración aplicada protege el archivo `.up.sql`, que es el que determina el estado de la base. Por eso el `.down.sql` de `000003` se corrigió el 2026-11-18 sin tocar su `.up.sql`:

- Revocaba `EXECUTE` sobre funciones que el propio archivo acababa de eliminar, así que la reversión fallaba.
- Revocaba el `SELECT` del rol runtime, que el bootstrap había concedido, y dejaba la API sin poder leer.
- Ahora archiva la traza de auditoría en `public.admin_audit_log_archive_000003` antes de eliminar el esquema `app`, porque una reversión no puede borrar la evidencia de quién cambió las membresías. La reversión de `000004` hace lo mismo con `public.admin_audit_log_archive_000004`.

Ambas reversiones están cubiertas por pruebas de integración reales (`TestIntegrationDownMigrationsExecute` y `TestIntegrationAdminWriteRollbackArchivesTheAuditTrail`).
