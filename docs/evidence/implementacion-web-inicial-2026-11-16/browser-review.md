# Revisión de navegador — implementación web inicial

Fecha: 2026-11-16

## Evidencia ejecutada

- Servidor: `pnpm dev` desde `apps/web`.
- URL: `http://localhost:3000/`.
- Viewports observados por el navegador integrado: móvil `366 × 812` y escritorio `1096 × 1265`.
- `document.documentElement.scrollWidth`: `351`; no hubo desbordamiento horizontal.
- Consola: cero errores.
- Lighthouse final en ambos viewports: accesibilidad `100`, buenas prácticas `100`, SEO `100`.
- La auditoría final no reportó fallos de contraste ni de nombres accesibles.
- Secuencia de teclado: el primer `Tab` enfoca «Saltar al contenido» y los siguientes recorren marca, «Vista general», «Dispositivos» y «Acerca de», todos con contorno visible.

## Contenido comprobado

- La pantalla expone «MODO DEMO», «DATOS DE EJEMPLO» y «No hay equipos conectados».
- «Estado de red» aparece como `—` y «Sin datos disponibles`.
- El contador de dispositivos es `00` y se muestra el estado vacío.
- Los enlaces de navegación apuntan a `#inicio`, `#dispositivos` y `#acerca`.
- Se guardaron `mobile-browser.png`, `desktop-browser.png` y los informes Lighthouse en esta carpeta.

## Adaptación

- Las capturas `mobile-browser.png` y `desktop-browser.png` cubren los viewports móvil y escritorio.
- La hoja de estilos contiene los cortes explícitos para escritorio/tableta (`max-width: 900px`) y móvil (`max-width: 600px`); el build de producción pasó.
- Las pruebas en teléfonos físicos siguen pendientes de la validación posterior solicitada.
