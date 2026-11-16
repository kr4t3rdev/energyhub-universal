# Evidencia — validación visual Bootstrap 00

Fecha: 2026-11-15  
Repositorio: `/Volumes/Server/Proyectos/energyhub-universal`

## Estado

- **Web:** lint, build, arranque HTTP, DOM y Lighthouse comprobados. La aplicación es todavía la plantilla inicial de Next.js. La revisión en `1440×900`, `768×1024` y `390×844`, la navegación completa por teclado y las capturas quedan pendientes porque el navegador integrado no expuso una pestaña visible.
- **iOS:** build para `iphonesimulator` correcto con rutas de derivados dentro de `.local/`; bloqueado para ejecución y tests por ausencia de runtime descargable.
- **Android:** build y prueba unitaria correctos con Gradle local; bloqueado para ejecución visual porque no hay dispositivo/emulador y la descarga del emulador/imagen falló por red.

## Archivos

- `preparation/tool-versions.log`, `preparation/disk-space.log`: versiones y espacio sin credenciales.
- `web/pnpm-install.log`, `web/pnpm-lint.log`, `web/pnpm-build.log`, `web/next-start.log`: instalación, lint, build y servidor.
- `web/http-headers.log`, `web/http-body.html`, `web/content-check.log`: arranque y contenido servido.
- `web/browser-dom-evidence.md`: resultado de la inspección DOM, consola, overflow, tema y foco, con limitaciones de captura.
- `web/lighthouse-retry.json`, `web/lighthouse-retry.html`: informes Lighthouse del reintento.
- `web/screenshots/capture-attempt.md`: intentos de captura con Brave headless y GUI, ambos bloqueados por el entorno.
- `ios/simctl-runtimes.log`, `ios/xcode-download-platform-ios.log`: runtime/dispositivos y resultado del intento de descarga oficial.
- `ios/xcode-show-build-settings.log`, `ios/xcodebuild-build.log`, `ios/xcodebuild-test.log`: settings por defecto, build con rutas fijadas y test bloqueado.
- `ios/runtime-retry-summary.md`: intento de descargar iOS 26.1 y motivo del timeout.
- `android/adb-devices.log`, `android/avd-list.log`, `android/sdkmanager-*.log`: dispositivos, AVD e intento de preparar imagen oficial.
- `android/runtime-retry-summary.md`: reintentos de SDK/Emulator y descarga directa oficial.
- `android/gradle-test-assemble.log`, `android/apk-badging.log`: prueba unitaria, APK y metadatos.

No se incluyen capturas de pantalla en esta ejecución: no se pudo obtener una captura fiable sin una pestaña visible.
