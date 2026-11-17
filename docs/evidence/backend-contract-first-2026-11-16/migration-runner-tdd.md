# Evidencia TDD — runner de migraciones

## RED

`go test ./internal/storage/postgres` no compiló antes de implementar el runner:

```text
undefined: Migration
undefined: NewMigrationStore
undefined: ErrMigrationStoreUnavailable
```

## Decisión

- El runner es un comando explícito (`cmd/migrate`); la API nunca aplica migraciones.
- `fs.ReadDir` sustituye a `fs.Glob`, que ignora errores de E/S y podía ocultar un directorio ilegible.
- Cada archivo se valida por nombre (`NNNNNN_name.up.sql`), cuerpo no vacío, versión única y orden ascendente.
- Una versión aplicada en la base que el binario no conoce aborta el run: un binario antiguo no debe operar contra un esquema más nuevo.
- El SQL de cada migración se envía con `QueryExecModeSimpleProtocol` porque un archivo puede contener varias sentencias.
- La migración y el registro en `schema_migrations` ocurren en una transacción; cualquier error hace rollback.
- Los `.down.sql` quedan como referencia manual y no se automatizan.

## GREEN

Comportamiento cubierto:

- carga ordenada de archivos `.up.sql`, ignorando `.down.sql` y documentación;
- rechazo de nombres inválidos, cuerpos vacíos, versiones duplicadas y planes desordenados antes de tocar la base;
- aplicación solo de versiones pendientes y omisión de las ya aplicadas;
- detención en el primer fallo, conservando el resultado parcial;
- rechazo de versiones desconocidas en la base;
- creación de `schema_migrations` y error si no hay permisos;
- lectura de versiones aplicadas, incluyendo errores de consulta, `Scan` y `Rows.Err`;
- transacción por migración con commit, rollback en error de SQL, de registro o de commit, y error al abrir la transacción;
- guardas para store nulo o base nula.

## Verificación

- `go test ./internal/storage/postgres`: PASS, cobertura 100%.
- `go test ./...`, `go vet ./...`, `go build ./...`, `go mod verify`: PASS.
- `go test -race ./internal/storage/postgres ./internal/authz`: PASS.
- Sin `DATABASE_URL`, `go run ./cmd/migrate` falla con `DATABASE_URL is required` y no abre conexión.
- No se ejecutó ninguna migración contra una base real ni se introdujeron credenciales.
- Notion no se actualizó porque el conector no está disponible en esta sesión.
