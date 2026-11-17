# Evidencia — RLS y administración de membresías

## Diseño aplicado

- `000002_enable_tenant_rls` habilita y fuerza RLS en `organizations` y `organization_memberships`.
- Lectura: un subject solo ve sus propias filas de membresía y las organizaciones donde es miembro.
- Escritura: solo se permite con `app.is_system_admin = 'on'`; sin esa variable, toda escritura queda denegada.
- `SYSTEM_ADMIN_SUBJECT` define el único subject autorizado. La autorización falla cerrada si está vacío.
- `AdminMembershipService` comprueba el `sub` verificado antes de validar el objetivo y antes de tocar el writer.
- El lector fija `app.current_subject` dentro de la misma transacción de lectura; el writer fija `app.current_subject` y `app.is_system_admin` dentro de su transacción.

## Pruebas

- Migración: se lee el SQL y se verifican `ENABLE`/`FORCE ROW LEVEL SECURITY`, las cuatro políticas, las variables de sesión y la ausencia de `USING (true)`. El down elimina las políticas y desactiva RLS.
- Lector: transacción, `set_config`, consulta, commit y rollback en errores de `Begin`, `set_config`, consulta, `Scan`, `Rows.Err` y commit.
- Writer: `set_config` de subject y admin, upsert/delete, commit y rollback; validación de admin, organización, subject objetivo y rol antes de abrir la transacción. El writer vuelve a comparar el admin configurado como defensa en profundidad.
- Servicio admin: un principal distinto del configurado se rechaza antes de llamar al writer; configuración vacía y writer ausente fallan cerrados; se aceptan los cuatro roles del contrato.
- Cobertura: `authz` 100%, `storage/postgres` 100%.

## Límite explícito

No se ejecutó una instancia PostgreSQL real. Las políticas están definidas y las variables se fijan en transacción, pero el aislamiento efectivo de RLS no está verificado empíricamente. No se declara producción-ready ni se afirma que RLS bloquee accesos sin una prueba contra PostgreSQL.

## Verificación

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `go mod verify`: PASS.
- `go test -race ./internal/authz ./internal/storage/postgres`: PASS.
- Notion no se actualizó porque el conector no está disponible en esta sesión.
