# Reintento de emulador Android — 2026-11-15

- `sdkmanager` se reintentó para `emulator` y `system-images;android-35;google_apis;arm64-v8a`; la descarga quedó atascada en el paquete Emulator y agotó el timeout.
- Se intentó descargar directamente el archivo oficial de Emulator para macOS arm64 (`452,156,012` bytes); la transferencia se reinició/interrumpió por timeout y no se instaló. El archivo parcial creado durante el intento fue retirado.
- El AVD del perfil del usuario no contiene una imagen reutilizable y permanece sin tocar.

Resultado: no hay emulador ni imagen instalable en el SDK del proyecto; no se pudo instalar ni abrir el APK.
