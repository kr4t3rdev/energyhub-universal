# Propuesta: validación visual de Bootstrap 00

## Problema y objetivo

Las tres interfaces compilan, pero falta evidencia de que se ejecuten y presenten su contenido inicial en un navegador y dispositivos simulados o físicos. Validar visualmente web, iOS y Android, documentando entorno, resultados y bloqueos sin declarar implementadas funciones de producto.

## Alcance

- Preparar herramientas locales y espacio; usar el volumen externo para el proyecto, herramientas Android y evidencias. Xcode administra sus runtimes de iOS mediante macOS; registrar dónde se instalan.
- Ejecutar web, iOS y Android y comprobar arranque, contenido real, legibilidad, apariencia, accesibilidad básica y ausencia de fallos que impidan el uso.
- Guardar capturas y registro de pruebas sin secretos; documentar dispositivo, versión, comandos y resultados. Corregir solo fallos necesarios para el arranque o visualización y volver a comprobarlos.
- Mantener el historial existente y actualizar el plan de Notion si el conector está disponible.

## Fuera de alcance

Rediseño del producto, login, facturación, conectores, telemetría, cambios a PostgreSQL compartido, despliegues y promesas de compatibilidad de hardware. No introducir dependencias o configuraciones globales innecesarias. No cambiar proyectos ajenos ni sobrescribir archivos del usuario.

## Estado conocido, sujeto a verificación

- Web: Next.js muestra plantilla inicial; `pnpm lint` y `pnpm build` pasaron previamente.
- iOS: SwiftUI muestra «EnergyHub Universal»; compilación anterior exitosa; al inspeccionar no se listaron runtimes.
- Android: Compose muestra «EnergyHub Universal»; SDK 35 y Gradle 8.11.1 locales; compilación y prueba unitaria previas exitosas con caché. No se detectaron emulador ni imagen de sistema en el SDK local.
- PostgreSQL local 17.11 respondió en 5433; estas pantallas no dependen de su conexión. Notion ha estado inaccesible.

## Restricciones y riesgos

Las descargas pueden ser grandes o lentas; comprobar espacio, disponibilidad y origen oficial. Xcode previamente escribió en una carpeta compartida: fijar `SYMROOT`, `OBJROOT` y `-derivedDataPath` dentro del proyecto, y revisar configuración de build antes de ejecutar. Emuladores no demuestran funcionamiento en hardware real. Evitar exponer variables de entorno o credenciales.

## Entregables y aceptación

El cambio queda completo solo cuando cada plataforma tiene resultado trazable (aprobada o incidencia resuelta), capturas y entorno registrados; una plataforma bloqueada mantiene abierto el cambio. Ver [tareas](tasks.md) y [especificación](specs/validacion-visual/spec.md). Sin publicar ni hacer push. No crear commit salvo autorización expresa o instrucciones vigentes aplicables.
