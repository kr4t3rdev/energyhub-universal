# Resumen para revisión — implementación web inicial (2026-11-19)

Cierra la tarea 5.6 de `openspec/changes/implementacion-web-inicial/tasks.md`.
Sin push, despliegue ni publicación: eso lo hace el usuario desde su Terminal
cuando termine el otro agente.

## Alcance entregado

- Vista general en español con datos sintéticos etiquetados (`apps/web`,
  Next.js 16.4.6): lecturas, frescura, estados vacíos y sin datos.
- Panel de Dispositivos (`cb96827`): `DeviceSummary` con `kind` y `status`
  nullable, como propone `contracts/openapi/next-phase-proposal.yaml`;
  2 dispositivos de ejemplo (Inversor solar en línea, Batería doméstica
  sin datos). Con lista vacía se mantiene el estado vacío original.
- Frontera intacta: `apps/web/app/site/provider.ts` sigue con la fixture;
  no hay peticiones al backend, ni auth, ni billing, ni equipos reales.
  `/health` sigue siendo solo liveness y no se usa como dato.

## Verificación (en el Mac, 2026-11-18)

- `pnpm test`: 16/16 en verde (12 iniciales + 4 de dispositivos, TDD con RED observado).
- `pnpm exec tsc --noEmit`, `pnpm lint`, `pnpm build`: correctos.
- Smoke del build servido: el HTML trae "Inversor solar", "Batería doméstica",
  "DATOS DE EJEMPLO" y "Valor de ejemplo".
- Lighthouse del 2026-11-16: accesibilidad 100, buenas prácticas 100, SEO 100.

## Fuera de este resumen (no tocado aquí)

- Backend del otro agente: `def2f58` commiteado por él; quedan sin commitear
  `backend/scripts/dev-stack.sh` y la migración nueva
  `backend/migrations/000006_close_admin_audit_read.up.sql` (sin `.down.sql`
  todavía), editados por él minutos antes de este resumen. No se commitean aquí.
- Endpoints de la siguiente fase (`GET /v1/me`, `/sites`, `/overview`,
  `/devices`): diseñados en `docs/architecture/backend-next-phase.md`,
  sin rutas ni persistencia.

## Siguiente fase propuesta

1. Cuando existan los endpoints, sustituir `FixtureSiteOverviewProvider`
   por un cliente de la API real (misma frontera, mismos tipos de presentación).
2. Después, llevar la misma vista a iOS/Android con la fixture, antes de
   conectarlas a la API.
3. Pruebas en teléfonos físicos iOS/Android y revisión visual (pendientes
   desde Bootstrap 00).
4. Push de los 17 commits locales, por el usuario cuando termine el otro agente.
