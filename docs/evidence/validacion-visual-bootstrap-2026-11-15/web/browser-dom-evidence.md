# Evidencia web — inspección de arranque y DOM

Fecha: 2026-11-15  
Servidor: `pnpm start --hostname 127.0.0.1 --port 3000`  
Navegador: Chromium integrado de OpenCode; viewport reportado por la sesión: `1000×700`.

## Resultado observado

- HTTP de la aplicación compilada: `200 OK`; respuesta conservada en `http-body.html`.
- Contenido real: plantilla inicial de Next.js (`Create Next App`, `To get started, edit the page.tsx file.`, enlaces `Templates`, `Learning`, `Deploy Now` y `Documentation`).
- Consola del navegador durante la carga: cero errores (`browser.console(level=error)`).
- Desbordamiento horizontal en el viewport observado: no (`scrollWidth=1000`, `clientWidth=1000`).
- Preferencia de tema observada: oscuro (`prefers-color-scheme: dark=true`); fondo calculado `rgb(10, 10, 10)` y texto `rgb(237, 237, 237)`.
- Foco: al enfocar un enlace, el navegador mostró el contorno nativo `rgb(229, 151, 0) auto 1px`. La secuencia automatizada de `Tab` no avanzó desde `BODY` en esta sesión, por lo que la navegación completa por teclado queda pendiente.

## Reintento — 2026-11-15

- El navegador integrado volvió a conectarse y cargó la aplicación compilada en `http://127.0.0.1:3000/`.
- Lighthouse integrado terminó sin fallos y produjo puntuaciones `100/100` en accesibilidad, buenas prácticas y SEO. Los informes están en `lighthouse-retry.json` y `lighthouse-retry.html`.
- La captura volvió a fallar con `browser.screenshot failed: Screenshot needs a visible tab`; la sesión no expone una pestaña visible aunque el DOM sí sea accesible.
- Se repitió la comprobación de teclado: `Tab` no avanzó desde el primer enlace en el transporte del navegador. No se convierte en aprobación de navegación por teclado.

## Limitaciones

- El navegador integrado no permitió generar capturas: `browser.screenshot` requirió una pestaña visible. En el reintento el DOM estuvo conectado, pero la pestaña siguió sin estar disponible para captura.
- La sesión no tenía Playwright, Chromium de línea de comandos ni otra herramienta local de captura instalada. No se añadieron dependencias solo para producir capturas.
- No se pudieron fijar los viewports solicitados de `1440×900`, `768×1024` y `390×844`; únicamente queda validado el viewport reportado arriba. No se marca completa la revisión multidispositivo ni se adjuntan capturas.
