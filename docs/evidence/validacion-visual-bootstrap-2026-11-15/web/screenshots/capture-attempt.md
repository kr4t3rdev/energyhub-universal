# Intento de capturas web — 2026-11-15

Se intentaron capturas reales con dos rutas sin modificar la aplicación:

1. Brave headless con `--window-size=1440,900`, `768,1024` y `390,844`; el proceso no produjo PNG y quedó bloqueado por `CVDisplayLinkCreateWithCGDisplay failed` en este entorno sin display.
2. Brave GUI con un perfil aislado bajo `.local/`; la ventana pudo solicitarse, pero macOS rechazó la consulta de sus límites con `osascript`: `System Events ... no tiene permitido el acceso de ayuda (-1719)`.

No se inventaron ni se guardaron imágenes que no fueran capturas de la aplicación. Los logs de ambos intentos están en esta carpeta. El servidor temporal se cerró mediante `trap` y se verificó que el puerto `3000` quedó detenido.
