# Diseño — Aislamiento de lectura por rol de PostgreSQL por tenant

Fecha: 2026-11-20
Estado: aprobado por el usuario (frontera de lectura por rol de tenant)

## Contexto

Hoy `app.current_subject` es un GUC de sesión que **cualquier rol con la credencial de la API puede falsificar** (`SET app.current_subject='otro-subject'`). Aunque `VerifyRuntimeRole` protege al arranque, una vez dentro de la sesión un sujeto malintencionado o una inyección SQL puede leer datos de otro tenant.

El objetivo es eliminar la dependencia del GUC para lecturas de datos de tenant usando un **rol de PostgreSQL por organización** y `SET ROLE` antes de cada consulta de lectura.

## Objetivo

- Que la credencial de la API (`energyhub_runtime`) no pueda leer directamente las tablas de tenants
- Que para leer datos de una organización específica, la sesión asuma el rol de ese tenant (`SET ROLE energyhub_tenant_<org_id>`)
- Que cada rol de tenant solo pueda leer los datos de su propia organización
- Mantener `ListOrganizations` funcionando sin GUC (consulta directa con `WHERE subject = $1`)

## Componentes

### 1. Migración `000007_tenant_isolation_roles`

**`.up.sql`:**
- Crea una función `app.tenant_role_name(p_organization_id text)` que convierte un `organization_id` en un nombre de rol válido de PostgreSQL (`energyhub_tenant_<sanitizado>`)
- Crea una función `app.set_tenant_role(p_organization_id text)` que:
  1. Verifica que el subject autenticado pertenece a la organización (consulta `organization_memberships`)
  2. Ejecuta `SET ROLE energyhub_tenant_<org_id>`
- Concede a `energyhub_runtime` EXECUTE en `app.set_tenant_role`
- Crea roles de tenant para organizaciones existentes (`energyhub_tenant_<org_id>`)
- Agrega `energyhub_runtime` como miembro de cada rol de tenant
- Configura RLS en `public.sites` y `public.site_devices` para filtrar por `organization_id`

**`.down.sql`:** Fail-closed: verifica que no queden roles de tenant huérfanos y que el estado sea consistente.

### 2. Cambios en `membership.go`

`ListOrganizations` ya no usa `set_config('app.current_subject', ...)`. Consulta directamente con `WHERE m.subject = $1`. El GUC se elimina completamente de este camino.

### 3. Cambios en `site.go`

`SiteReader.beginScoped` deja de usar `setMembershipSubjectQuery`. En su lugar, antes de ejecutar consultas de lectura de datos de tenant, ejecuta `SELECT app.set_tenant_role($1)` con el `organization_id` del scope. Al finalizar la transacción, ejecuta `RESET ROLE`.

### 4. Cambios en `cmd/api/main.go`

No requiere cambios significativos: el pool sigue siendo el mismo (`energyhub_runtime`). El `SET ROLE` se gestiona dentro de cada transacción de lectura de sitio.

### 5. Pruebas de integración

- `TestIntegrationTenantRoleIsolation`: verifica que `energyhub_runtime` no puede leer datos de un tenant sin hacer `SET ROLE`
- `TestIntegrationTenantRoleSetAndReset`: verifica que `SET ROLE` permite leer y `RESET ROLE` revoca el acceso
- `TestIntegrationTenantRoleName`: verifica la función de conversión de nombre de rol
- Actualizar `site_test.go` para probar el nuevo flujo con `SET ROLE`

## Límites conocidos

- Los nombres de rol de PostgreSQL tienen restricciones de caracteres; `organization_id` se sanitiza (guiones → guiones bajos)
- Crear un rol por organización requiere que `energyhub_owner` (dueño de `create_organization`) cree el rol y agregue `energyhub_runtime` como miembro
- `SET ROLE` es por sesión/transacción; se revierte con `ROLLBACK` y debe hacerse explícitamente con `RESET ROLE` antes de `COMMIT`

## Pendiente tras la implementación

- Crear automáticamente el rol de tenant cuando se crea una organización (en `app.create_organization`)
- Crear las tablas `public.sites` y `public.site_devices` con RLS por organización
- Extender el aislamiento a lecturas de lecturas y dispositivos