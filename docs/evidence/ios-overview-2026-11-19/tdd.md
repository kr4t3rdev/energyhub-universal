# TDD — vista general demo en iOS (2026-11-19)

Espejo en SwiftUI de la web/Android, sobre la misma fixture
("Casa de ejemplo", 4 lecturas, 2 dispositivos). Sin backend, sin auth,
sin afirmar equipos reales. Xcode 27, Swift 6.4, iOS 17.0, bundle
`com.energyhub.universal`.

## Archivos

- `apps/ios/EnergyHub/Site/SiteOverview.swift` — `Reading`,
  `DeviceSummary` (`kind`, `status: String?`), `SiteOverview` y la fixture
  `exampleSite`, mismos datos y textos que `apps/web/app/site/example.ts`.
- `apps/ios/EnergyHub/Site/SitePresentation.swift` — lógica pura sin
  SwiftUI: `formatReading` (valor con unidad en español o "—" si nil),
  `deviceStatusLabel` (estado o "Sin datos disponibles") y `hasDevices`.
  Nil nunca se muestra como cero/apagado/ahorro.
- `apps/ios/EnergyHub/ContentView.swift` — título, banner
  "DATOS DE EJEMPLO", lecturas y dispositivos; estado vacío si la lista es [].
- `apps/ios/EnergyHubTests/SitePresentationTests.swift` — 10 tests XCTest
  (lectura con/sin valor, dispositivo con/sin estado, lista vacía/no vacía,
  contenido de la fixture). `SmokeTests` queda intacto.
- `apps/ios/EnergyHub.xcodeproj/project.pbxproj` — regenerado con
  `xcodegen generate` (2.46.0, en `/opt/homebrew/bin`); diff de +20 líneas,
  solo las referencias a los archivos nuevos.

## RED observado

`build-for-testing` con destino `generic/platform=iOS Simulator` falló
con `cannot find 'formatReading' in scope`, `cannot find 'DeviceSummary'
in scope`, `cannot find 'deviceStatusLabel' in scope` (`tests-red.log`).
Nota: un primer intento falló antes por un error de sintaxis mío en el
test (`\u2014` no es válido en Swift, debe ser el carácter literal "—");
se corrigió el test y el RED registrado es el real, por identificadores
ausentes.

## GREEN (compilación)

- `xcodebuild ... build` (comando documentado en `docs/environment.md`):
  **BUILD SUCCEEDED** (`build.log`).
- `xcodebuild ... build-for-testing` con destino genérico:
  **TEST BUILD SUCCEEDED** (`tests-green.log`); los 10 tests nuevos más
  el `SmokeTests` compilan. `EnergyHub.app` generado en
  `.local/DerivedData/ios-build-products/Debug-iphonesimulator/`.

## Ejecución de tests: NO posible desde SSH (no fingida)

- Novedad: `xcrun simctl list runtimes` YA muestra el runtime
  iOS 26.1 (26.1 - 23B80) con dispositivos (iPhone 17, iPhone 16e, iPad...).
  Alguien lo instaló después del 2026-11-15; no se descargó nada en esta
  tranche.
- `xcodebuild test` con destino genérico: "Tests must be run on a
  concrete device" (`test-run-attempt.log`).
- `xcodebuild test` con el iPhone 16e concreto: **TEST FAILED** por
  "Unable to boot the Simulator" (`test-run.log`).
- Boot explícito de iPhone 16e y de iPhone 17: mismo error —
  "Failed to start launchd_sim: could not bind to session,
  launchd_sim may have crashed or quit responding" (NSPOSIXErrorDomain
  code=4, Interrupted system call). Es un fallo del entorno al arrancar
  el simulador desde la sesión SSH, igual en los dos dispositivos.
- Por tanto: los tests están escritos y compilan, pero su ejecución
  queda pendiente de una sesión con simulador funcional o de un
  teléfono físico. No se afirma ningún test ejecutado en verde.
