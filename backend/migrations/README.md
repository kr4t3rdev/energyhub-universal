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
