# Reintento de runtime iOS — 2026-11-15

- `xcodebuild -downloadPlatform iOS -buildVersion 26.0 -architectureVariant arm64`: Xcode indicó que iOS 26.0 no estaba disponible.
- `xcodebuild -downloadPlatform iOS -buildVersion 26.1 -architectureVariant arm64`: Xcode sí encontró iOS 26.1 Simulator arm64, tamaño anunciado `8.32 GB`, pero la descarga alcanzó aproximadamente `47 MB` en 10 minutos y fue detenida por timeout.
- Después del intento, `xcrun simctl list runtimes` sigue sin runtimes instalados.

El bloqueo actual es la descarga extremadamente lenta/incompleta, no falta de espacio: el volumen de Xcode tenía aproximadamente `28 GiB` libres después del intento.
