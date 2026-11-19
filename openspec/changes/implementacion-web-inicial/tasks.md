# Tareas — implementación inicial de la web EnergyHub

Estado: implementación web inicial completada para revisión. La preparación inicial del repositorio se considera cerrada por decisión del usuario. Las pruebas móviles en teléfonos físicos quedan pendientes. Esta fase crea una base web demostrable; no declara compatibilidad con hardware ni disponibilidad de datos reales.

## Avance de implementación

La vista web en español ya incluye instalación, lecturas sintéticas señalizadas, estado vacío de dispositivos, navegación por anclas y estilos responsive. Los colores y la identidad son propuestas provisionales. No hay integración con equipos ni API de datos; `/health` únicamente informa de vida del proceso. Se leyó el plan maestro de Notion; el primer intento de actualizarlo falló con `ECONNRESET`, pero la actualización final quedó confirmada.

La fixture local está en `apps/web/app/site/example.ts`, la frontera en `apps/web/app/site/provider.ts` y el formateo/frescura en `apps/web/app/site/presentation.ts`. La página usa componentes de servidor. La propuesta de autenticación, aislamiento tenant y contratos está en `docs/architecture/backend-next-phase.md`; el OpenAPI propuesto está marcado como no implementado en `contracts/openapi/next-phase-proposal.yaml`. Las pruebas de presentación y la revisión de navegador están registradas en `docs/evidence/implementacion-web-inicial-2026-11-16/`.

Referencias: `README.md`, `docs/environment.md`, `openspec/changes/validacion-visual-bootstrap/tasks.md`, `contracts/openapi/health.yaml` y el plan maestro de Notion cuando esté accesible. Proyecto en el volumen externo; preservar los archivos existentes, incluidos los no rastreados. Registrar decisiones y resultados en la documentación existente.

## 0. Alcance y decisiones de producto

- [x] 0.1 Leer el plan maestro de Notion si está disponible y conciliar sus decisiones con el repositorio; el primer intento de escritura falló con `ECONNRESET`, pero el registro final quedó confirmado.
- [x] 0.2 Definir un primer recorrido web demostrable: entrada, vista general de una instalación de ejemplo y estados sin dispositivos/datos. La web es centro de análisis y administración; la captura de equipos corresponde a móviles en una fase posterior.
- [x] 0.3 Distinguir decisiones confirmadas de propuestas visuales: Cuba primero, futura expansión internacional; producto multi-tenant con suscripción mensual; Next.js + Go + OpenAPI. Precios, proveedor de pagos, marca visual definitiva y datos reales siguen sin decidirse.
- [x] 0.4 Documentar el alcance de la primera entrega y cualquier elección de diseño; evitar implementar autenticación, facturación, telemetría real o conectores durante esta fase inicial sin alcance adicional.

## 1. Fundamentos y herramientas

- [x] 1.1 Revisar estructura existente de App Router, TypeScript, Tailwind y ESLint; conservar pnpm y su lockfile. Ejecutar instalación reproducible, lint y build para establecer línea base.
- [x] 1.2 Definir estructura por funcionalidad, límites entre componentes de servidor y cliente, rutas y convenciones de datos. Usar componentes simples antes de añadir bibliotecas.
- [x] 1.3 Configurar estrategia de pruebas web acorde al proyecto: Vitest 5.1.0 para reglas de presentación y smoke/auditoría en navegador. Dependencias y lockfile quedaron fijados.
- [x] 1.4 Establecer configuración de entorno documentada, sin credenciales en el repositorio; usar variables del servidor cuando proceda. El endpoint `/health` indica vida del proceso, no estado de PostgreSQL.

## 2. Diseño y experiencia

- [x] 2.1 Establecer dirección visual inicial para EnergyHub y tokens básicos de color, tipografía, espaciado, bordes y estados. La identidad visual es propuesta, no decisión previa del usuario.
- [x] 2.2 Crear estructura responsive para móvil, tableta y escritorio: navegación clara, cabecera, contenido principal y rutas relevantes; mantener el acceso por teclado y foco visible.
- [x] 2.3 Sustituir la plantilla predeterminada por una página inicial propia con nombre de producto y descripción fiel al estado del proyecto, sin botones que prometan funciones no implementadas.
- [x] 2.4 Preparar componentes reutilizables necesarios para la primera vista: tarjeta de resumen, estado vacío, indicador de estado y mensajes de error/carga. Probar comportamiento y accesibilidad relevantes.
- [x] 2.5 Revisar contraste, jerarquía de encabezados, nombres accesibles, tamaños de texto y movimiento reducido; Lighthouse final obtuvo 100/100/100.

## 3. Vista de demostración energética

- [x] 3.1 Definir un modelo local de ejemplo tipado para instalación, dispositivos y lecturas mínimas, con unidades y fecha; usar fixtures explícitamente sintéticas y separadas de adaptadores futuros.
- [x] 3.2 Mostrar vista general con métricas disponibles en la fixture, estado de los dispositivos y última actualización. Etiquetar de forma visible «datos de ejemplo»; no simular conexión real ni certificación de marcas.
- [x] 3.3 Representar valores faltantes, obsoletos y sin datos sin inventarlos; diferenciar valores medidos de estimados si la fixture incluye ambos. No inferir apagones, autonomía, salud o ahorro sin datos y reglas respaldadas.
- [x] 3.4 Mantener solo estados que tienen comportamiento real en esta entrega: demo, vacío y datos ausentes; no se añadieron controles ficticios de carga/error.
- [x] 3.5 Añadir pruebas primero para las reglas de presentación de datos relevantes (unidades, faltantes y frescura); implementar y ejecutar con evidencia RED/GREEN en `docs/evidence/implementacion-web-inicial-2026-11-16/tdd.md`.

## 4. Contratos y preparación de integración

- [x] 4.1 Inventariar el contrato OpenAPI existente: actualmente solo `/health`; no inventar endpoints ni utilizar health como indicador de disponibilidad de datos.
- [x] 4.2 Definir interfaces de acceso a datos y frontera para sustituir fixtures por API en una fase posterior, sin introducir peticiones ficticias ni acoplar componentes a datos de fabricante.
- [x] 4.3 Documentar contratos faltantes para sitios, dispositivos y resumen; no implementarlos hasta acordar autenticación, ámbito tenant, errores y permisos. Toda futura lectura de datos reales deberá aislarse por tenant.
- [x] 4.4 Mantener PostgreSQL local actual intacto y no asumir que la API ya consulta la base de datos.

## 5. Validación y entrega

- [x] 5.1 Ejecutar lint, pruebas añadidas, verificación de tipos y build de producción; registrar comandos, versiones y resultados sin secretos.
- [x] 5.2 Iniciar web local y revisar escritorio, tableta y móvil, incluido teclado, foco, recortes, errores de consola y modos aplicables; las capturas y auditorías de escritorio/móvil están documentadas. Las pruebas en teléfonos físicos siguen pendientes.
- [x] 5.3 Comprobar que la interfaz identifica datos sintéticos, presenta correctamente estados vacíos y faltantes y no afirma capacidades de hardware inexistentes.
- [x] 5.4 Revisar cambios y dependencias, fijar lockfile y corregir fallos; registrar riesgos y pruebas no ejecutadas.
- [x] 5.5 Actualizar el plan existente de Notion con avances y bloqueos si el conector funciona; la actualización del 2026-11-16 fue confirmada. Actualizar Second Brain solo si aporta información de seguimiento estable y sin duplicar la documentación canónica.
- [x] 5.6 Preparar resumen para revisión y siguiente fase; no hacer push, despliegue ni publicación sin autorización explícita. El commit inicial se gestiona por separado. Resumen en `docs/evidence/revision-web-inicial-2026-11-19/resumen.md` (2026-11-19).

## Fuera de alcance y pendientes posteriores

- Inicio de sesión, autorización y multi-tenancy reales; suscripciones y facturación.
- Integraciones reales de dispositivos, telemetría, analítica verificada, alertas y adquisición offline móvil.
- Pruebas iOS y Android en teléfonos físicos: ejecución, revisión visual y capturas aún pendientes; planificar instalación, firma y dispositivos cuando el usuario esté listo.
- Gradle Wrapper y bloqueo Android, CI remota, validación de contrato y despliegue: tareas posteriores según prioridad.
