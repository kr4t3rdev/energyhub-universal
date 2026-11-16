# Tareas — validación visual Bootstrap 00
    
Marcar únicamente cuando exista evidencia reproducible. Fecha, comandos, plataforma, versión, resultados, incidencias y rutas de capturas deben quedar en `docs/` o una subcarpeta de evidencias del proyecto. Evitar datos personales, variables y credenciales en capturas y logs.

## Estado de esta ejecución — 2026-11-15

`[x]` = comprobación completada con evidencia; `[B]` = bloqueada con motivo documentado; `[ ]` = pendiente. El cambio OpenSpec permanece abierto mientras haya plataformas bloqueadas o revisiones visuales pendientes.

- Preparación: completada.
- Web: compilación y arranque comprobados; revisión multidispositivo, capturas y navegación completa por teclado pendientes.
- iOS: build comprobado; runtime, ejecución, pruebas sobre dispositivo y revisión visual bloqueados por falta de runtime descargable.
- Android: build y prueba unitaria comprobados; emulador/dispositivo, instalación y revisión visual bloqueados por falta de imagen/emulador y fallos de red al descargar el SDK.
- Notion: no actualizado; la sesión no tiene un conector autenticado de edición.

Evidencia principal: `docs/evidence/validacion-visual-bootstrap-2026-11-15/`. No se generaron capturas porque el navegador integrado perdió la conexión y no había una herramienta local de captura disponible; no se afirma validación visual donde falta evidencia.

## 0. Preparación
- [x] 0.1 Revisar AGENTS.md, README.md, docs/environment.md, este cambio y estado Git; preservar todos los archivos no rastreados y cambios existentes.
- [x] 0.2 Confirmar que `/Volumes/Server/Proyectos/energyhub-universal` es accesible, espacio suficiente y versiones de Node/pnpm, Xcode, JDK, Gradle y SDK Android; comprobar runtimes y dispositivos disponibles.
- [x] 0.3 Definir una carpeta de evidencias dentro del repositorio; evitar incluir capturas privadas o binarios pesados en Git sin decisión explícita. Registrar bloqueos y alcance.
- [x] 0.4 Identificar la salida actual de cada app: plantilla Next.js en web; texto EnergyHub Universal en móviles. Comprobar de nuevo antes de usar como referencia.

## 1. Web
- [x] 1.1 Comprobar instalación desde lockfile; ejecutar `pnpm lint` y `pnpm build` en apps/web y registrar resultado.
- [x] 1.2 Arrancar la versión compilada localmente; abrirla en navegador y comprobar carga sin pantalla en blanco ni errores de consola que impidan el uso.
- [ ] 1.3 Revisar escritorio (ej. 1440×900), tableta (768×1024) y móvil (390×844), contenido visible, legibilidad, recortes y desplazamiento horizontal.
- [ ] 1.4 Revisar modos claro/oscuro si aplican y navegación por teclado con foco visible; capturar escritorio y móvil.
- [x] 1.5 Registrar navegador, dimensiones, resultado y errores. Detener el servidor de prueba al finalizar.

## 2. iOS
- [B] 2.1 Enumerar runtimes y simuladores con `xcrun simctl`; si no hay, comprobar espacio y preparar un runtime oficial compatible mediante Xcode, registrando versión y ubicación. Si no es posible, marcar bloqueado.
- [x] 2.2 Elegir iPhone simulado; compilar con firma desactivada y `-derivedDataPath`, `SYMROOT`, `OBJROOT` dentro de `.local/` del proyecto. Revisar build settings antes para evitar carpetas compartidas.
- [B] 2.3 Instalar y abrir la app, comprobar texto y ausencia de cierre inesperado; repetir arranque.
- [B] 2.4 Revisar modos claro/oscuro, áreas seguras y texto ampliado; registrar captura y dispositivo/OS.
- [B] 2.5 Ejecutar las pruebas iOS disponibles con destino concreto; registrar resultado independiente de la revisión visual.

## 3. Android
- [B] 3.1 Revisar SDK local, Gradle 8.11.1, JDK y dispositivos con adb; si no hay emulador o dispositivo, preparar herramientas e imagen oficial compatibles en el SDK del proyecto y crear AVD sin modificar configuraciones ajenas.
- [x] 3.2 Ejecutar `:app:testDebugUnitTest :app:assembleDebug` con Gradle local; especificar si las tareas se ejecutaron o reutilizaron caché.
- [B] 3.3 Instalar el APK en el dispositivo elegido y abrirlo; comprobar texto y que no se cierre; repetir arranque.
- [B] 3.4 Revisar claro/oscuro, barras del sistema y texto ampliado; registrar captura, dispositivo, versión y errores relevantes de logcat.
- [x] 3.5 Dejar Gradle Wrapper y bloqueo como trabajo separado salvo que sea estrictamente necesario para esta validación; no afirmar que la prueba unitaria existente verifica interfaz.

## 4. Cierre
- [x] 4.1 Clasificar por plataforma: aprobada, con incidencias o bloqueada. Corregir fallos de arranque/legibilidad dentro de alcance; verificar de nuevo y registrar evidencia.
- [x] 4.2 Actualizar documentación existente con fechas, comandos, resultados, limitaciones y rutas; no marcar completo ante bloqueos o fallos.
- [B] 4.3 Actualizar el plan Notion existente con resultados reales; si la integración falla, documentar el error y dejar pendiente sin fingir actualización.
- [x] 4.4 Resumir al usuario resultados, capturas, incidencias y siguientes pasos. No hacer push, publicar, desplegar ni crear un commit sin autorización apropiada.
