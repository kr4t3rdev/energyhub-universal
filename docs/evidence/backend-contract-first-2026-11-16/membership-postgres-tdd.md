# Evidencia TDD — membresías en PostgreSQL

## RED

El primer test del lector se escribió contra `pgxmock`. Faltaba el módulo:

```text
no required module provides package github.com/pashagolub/pgxmock/v4
```

Tras añadirlo, `pgxmock v4.9.0` no compiló contra `pgx v5.13.0`:

```text
*rowSets does not implement pgx.Rows (missing method TypeMap)
*batchResults does not implement pgx.BatchResults (missing method Tx)
```

Causa: la interfaz `pgx.Rows` ganó métodos en pgx v5.13 y el mock no los implementa. Degradar pgx o parchear el mock habría fijado una dependencia frágil.

## GREEN

Se eliminó `pgxmock` y se definió un puerto estrecho propio:

- `postgres.Rows`: `Next`, `Scan`, `Err`, `Close`.
- `postgres.Querier`: `Query(ctx, sql, args...)`.
- `postgres.NewQueryer` adapta `pgxpool.Pool` sin exponer pgx al lector.
- Una aserción de compilación (`var _ pgxQuerier = (*pgxpool.Pool)(nil)`) detecta futuros cambios de firma.

Comportamiento cubierto:

- el `subject` verificado se enlaza como `$1` y se recorta;
- la consulta filtra por la membresía del subject, no por un tenant del cliente;
- un resultado vacío devuelve lista no nula;
- roles inesperados en la base fallan con `ErrInvalidMembershipRole`;
- errores de consulta, `Scan` y `Rows.Err` se propagan;
- las filas se cierran en todos los caminos;
- subject vacío y lector no disponible se rechazan antes de consultar.

Las pruebas de migración leen el SQL y verifican `organization_id NOT NULL`, `subject NOT NULL`, la clave primaria compuesta, `ON DELETE CASCADE` y que el `CHECK` de roles coincide exactamente con `contracts.OrganizationRole`.

## Resultado

- `go test ./internal/storage/postgres`: PASS, cobertura 100%.
- `go test ./...`, `go vet ./...`, `go build ./...`, `go mod verify`: PASS.
- `go test -race ./internal/storage/postgres ./internal/authz`: PASS.
- No se ejecutó ninguna migración ni se abrió conexión a PostgreSQL.
- Notion no se actualizó en este ciclo porque el conector dejó de estar disponible en la sesión.
