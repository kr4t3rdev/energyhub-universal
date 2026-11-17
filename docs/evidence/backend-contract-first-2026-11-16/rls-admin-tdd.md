# Evidencia — RLS y administración de membresías

## Diseño aplicado

- `000002_enable_tenant_rls` habilita y fuerza RLS en `organizations` y `organization_memberships`.
- Lectura: un subject solo ve sus propias filas de membresía y las organizaciones donde es miembro.
- Escritura: `000003_secure_admin_writes` la restringe al rol de PostgreSQL `energyhub_membership_admin` y a las funciones `app.create_organization`, `app.grant_membership` y `app.revoke_membership` (`SECURITY DEFINER`, `search_path` vacío, `EXECUTE` revocado a `PUBLIC`).
- `SYSTEM_ADMIN_SUBJECT` define el único subject autorizado. La autorización falla cerrada si está vacío.
- `AdminMembershipService` comprueba el `sub` verificado antes de validar el objetivo y antes de tocar el writer.
- El lector fija `app.current_subject` dentro de la misma transacción de lectura. El writer llama a las funciones de admin con la credencial administrativa y nunca escribe en las tablas.

## Corrección del modelo anterior

La primera versión de esta evidencia describía la escritura administrativa con `app.is_system_admin = 'on'`. La verificación contra PostgreSQL 17.11 real mostró que esa frontera era falsificable:

- `SET app.is_system_admin='on'` como rol de la aplicación → permitido.
- `REVOKE SET ON PARAMETER "app.is_system_admin"` → aceptado por PostgreSQL, pero no restringe los GUC personalizados: el `SET` siguió funcionando.
- Con el flag puesto, el rol de la aplicación escribía membresías sin pasar por la autorización del servidor.

Por eso la escritura admin pasó a depender del rol de base de datos, no de un valor de sesión.

## Pruebas

- Migración `000002`: se lee el SQL y se verifican `ENABLE`/`FORCE ROW LEVEL SECURITY`, las cuatro políticas, las variables de sesión y la ausencia de `USING (true)`. El down elimina las políticas y desactiva RLS.
- Migración `000003`: se verifica que el SQL ejecutable no menciona `app.is_system_admin`, que las tres funciones son `SECURITY DEFINER` con `search_path` vacío, que `EXECUTE` se revoca a `PUBLIC`, que los propietarios y permisos apuntan a los roles correctos y que no hay `GRANT ALL`. El down elimina funciones y esquema.
- Lector: transacción, `set_config`, consulta, commit y rollback en errores de `Begin`, `set_config`, consulta, `Scan`, `Rows.Err` y commit.
- Writer: llamada a la función de admin, commit y rollback; validación de admin, organización, subject objetivo y rol antes de abrir la transacción; el writer vuelve a comparar el admin configurado como defensa en profundidad; ninguna llamada fija `app.is_system_admin`.
- Servicio admin: un principal distinto del configurado se rechaza antes de llamar al writer; configuración vacía y writer ausente fallan cerrados; se aceptan los cuatro roles del contrato.
- Cobertura: `authz` 100%, `storage/postgres` 100%.

## Verificación contra PostgreSQL real

`TestIntegrationTenantIsolationAndAdminWrites` (se activa con `TEST_DATABASE_URL`):

- el admin crea organización y membresía mediante `app.create_organization` y `app.grant_membership`;
- el rol runtime lee solo sus propias membresías y ve cero filas para otro subject;
- el rol runtime recibe `permission denied` al insertar en `organizations`;
- el rol runtime recibe `permission denied` al llamar a `app.grant_membership`;
- el rol runtime no obtiene lectura de administrador fijando `app.is_system_admin`;
- el rol admin también recibe `permission denied` al insertar directamente: solo puede escribir por las funciones;
- el rol admin lee todas las membresías por la rama de rol de la política;
- `app.revoke_membership` retira la membresía y el runtime deja de verla;
- las funciones admin rechazan rol inválido y organización vacía.

Ejemplo de ejecución:

```
TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55432/postgres' \
  go test ./internal/storage/postgres -run TestIntegration -v
```

## Límites explícitos

- El aislamiento de lectura sigue confiando en `app.current_subject`, un GUC que la credencial de la API puede fijar. La garantía es que el servidor lo deriva del token verificado y nunca de la petición del cliente; no es una frontera frente a una credencial de base de datos comprometida.
- La escritura administrativa sí está frente a la credencial: el rol runtime no puede escribir ni llamar a las funciones, y la API usa una credencial separada para administration.
- Las funciones admin se ejecutan con `BYPASSRLS`, por eso validan su entrada y repiten la comprobación del rol llamador.
- La prueba de integración crea roles de cluster con contraseña de desarrollo; no usar en un cluster compartido.
- No se declara producción-ready: falta la prueba contra el tenant Auth0 real y la política de rotación de credenciales.

## Verificación

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `go mod verify`: PASS.
- `go test -race ./internal/authz ./internal/storage/postgres`: PASS.
- `TEST_DATABASE_URL=... go test ./internal/storage/postgres -run TestIntegration`: PASS (9 subcasos).
- Notion no se actualizó porque el conector no está disponible en esta sesión.
