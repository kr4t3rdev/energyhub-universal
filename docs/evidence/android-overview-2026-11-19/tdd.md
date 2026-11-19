# TDD — vista general demo en Android (2026-11-19)

Mismos datos y reglas que la web (`apps/web/app/site/`): instalacion
"Casa de ejemplo", lecturas Potencia solar 1.24 kW, Consumo 0.86 kW,
Bateria 72 %, Estado de red sin datos; dispositivos Inversor solar
(inversor, En linea) y Bateria domestica (bateria, sin datos).
Todo etiquetado como datos de ejemplo. Sin equipos reales, sin fetch
al backend, sin auth.

## RED

Solo existia `SitePresentationTest` (10 tests) sin el modelo ni la
logica de presentacion: `:app:compileDebugUnitTestKotlin FAILED` con
`Unresolved reference` a `exampleSite`, `formatReading`,
`deviceStatusLabel` y `hasDevices`. Ver `tests-red.log`.

## GREEN

- `SiteOverview.kt`: `Reading`, `DeviceSummary` (`kind`, `status: String?`),
  `SiteOverview` y la fixture `exampleSite` (copia de la web).
- `SitePresentation.kt`: logica pura sin Compose — `formatReading`
  (valor con unidad en espanol o "—" si null), `deviceStatusLabel`
  (estado o "Sin datos disponibles") y `hasDevices`.
- `MainActivity.kt`: UI Compose (Material3) con titulo, banner
  "DATOS DE EJEMPLO", lecturas y dispositivos; estado vacio si la
  lista es []. `SmokeTest` original intacto.

## Verificacion (Mac, Gradle 8.11.1, JDK 21, SDK 35)

- `:app:testDebugUnitTest`: BUILD SUCCESSFUL — 11/11 tests en verde
  (10 `SitePresentationTest` + 1 `SmokeTest`, 0 fallos). Ver `tests-green.log`.
- `:app:assembleDebug`: BUILD SUCCESSFUL — APK
  `apps/android/app/build/outputs/apk/debug/app-debug.apk`, 9.281.394 bytes.
  Ver `build.log`.
- Sin emulador ni dispositivo: no se instalo ni se probo en pantalla.

## Fuera de alcance

Endpoints backend (`/v1/me`, `/sites`, `/overview`, `/devices`):
disenados, sin implementar; la app sigue con la fixture.
