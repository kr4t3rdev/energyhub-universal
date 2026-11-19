# Pantalla de Dispositivos sobre la fixture — 2026-11-18

Alcance: solo `apps/web` + esta carpeta de evidencia. Sin cambios en el
backend, sin fetch a la API, sin auth. La lista se alimenta de la fixture
sintética (`FixtureSiteOverviewProvider`); el banner "DATOS DE EJEMPLO" y el
texto "No hay equipos conectados" se mantienen.

## Cambio

- `apps/web/app/site/types.ts`: `DeviceSummary` suma `kind: string` y
  `status: string | null`, anticipando la propuesta de
  `contracts/openapi/next-phase-proposal.yaml` (id, name, kind, status
  nullable). `note` es solo presentación, como `icon`/`note` en `Reading`;
  no existe en el contrato del backend.
- `apps/web/app/site/example.ts`: 2 dispositivos de ejemplo, ambos con nota
  "Valor de ejemplo": "Inversor solar" (inversor, estado "En línea") y
  "Batería doméstica" (batería, `status: null`).
- `apps/web/app/site/presentation.ts`: `deviceStatusLabel(device)` devuelve
  el estado presente o "Sin datos disponibles" si es `null` o vacío;
  `hasDevices(devices)` indica si hay al menos uno.
- `apps/web/app/page.tsx`: el panel de Dispositivos lista nombre, kind,
  estado y nota cuando hay dispositivos; con lista vacía (`[]`) sigue el
  estado vacío original ("Aún no hay dispositivos"). Sin botones nuevos.
- `apps/web/app/globals.css`: estilos mínimos de la lista (misma paleta).

## TDD

- RED (`tests-red.log`): 4 tests nuevos fallaron, 12 pasaron (16 en total):
  `TypeError: deviceStatusLabel is not a function` y
  `TypeError: hasDevices is not a function`.
- GREEN (`tests-green.log`): 16/16 tests en verde.

## Verificación (desde `apps/web`, Node 22.23.4, pnpm 11.0.8)

- `pnpm test`: 16/16 en verde.
- `pnpm exec tsc --noEmit`: sin errores (`typecheck.log` vacío).
- `pnpm lint`: sin errores.
- `pnpm build`: correcto; `/` como prerender parcial.
- `pnpm install` no hizo falta: `node_modules` ya estaba instalado.

## Pendiente (fuera de esta tranche)

- Endpoints del backend (`GET /v1/me`, `/sites`, `/overview`, `/devices`):
  diseñados en `docs/architecture/backend-next-phase.md`, sin rutas ni
  persistencia; la web seguirá con la fixture hasta que existan.
- Pruebas en teléfonos físicos (iOS y Android) y revisión visual.
