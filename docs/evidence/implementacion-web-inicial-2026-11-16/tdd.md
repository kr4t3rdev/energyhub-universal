# Evidencia TDD — reglas de presentación web

Fecha: 2026-11-16

## Alcance

Las pruebas se derivaron de `openspec/changes/implementacion-web-inicial/tasks.md`, tarea 3.5:

- formatear unidades en español;
- representar valores ausentes sin inventar datos;
- clasificar lecturas como ausentes, frescas u obsoletas;
- producir etiquetas de antigüedad en minutos y horas.

## RED/GREEN

- RED: `presentation.test.ts` ejecutó 12 pruebas; 9 fallaron porque `readingFreshness` y `freshnessLabel` aún no existían. Las tres pruebas de `formatReading` pasaron.
- GREEN: `pnpm test` ejecutado desde `apps/web` pasó 12/12 pruebas.
- Cobertura: `pnpm exec vitest run --coverage` pasó 12/12; statements 100%, branches 95.83%, functions 100%, lines 100%.

## Garantías

| Garantía | Archivo | Resultado |
|---|---|---|
| `1.24` se muestra como `1,24 kW` | `app/site/presentation.test.ts` | PASS |
| `null` y `NaN` se muestran como `—` | `app/site/presentation.test.ts` | PASS |
| Una fecha ausente o inválida es `missing` | `app/site/presentation.test.ts` | PASS |
| Las lecturas dentro de 30 minutos son `fresh` | `app/site/presentation.test.ts` | PASS |
| Las lecturas fuera del umbral son `stale` | `app/site/presentation.test.ts` | PASS |
| La etiqueta comunica minutos, horas y obsolescencia | `app/site/presentation.test.ts` | PASS |

No se implementó una prueba E2E de interacción porque esta entrega no tiene acciones de negocio; el smoke de navegador y la auditoría de accesibilidad están documentados en `browser-review.md`.
