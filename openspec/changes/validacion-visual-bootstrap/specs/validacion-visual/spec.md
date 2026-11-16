# Especificación: validación visual inicial

## ADDED Requirements

### Requirement: Inicio y contenido de cada aplicación
Cada aplicación SHALL abrirse en su entorno de prueba y mostrar el contenido que corresponda a su estado actual, sin un cierre inesperado que impida la revisión.

#### Scenario: Inicio correcto
- **WHEN** se inicia una aplicación compilada en un navegador, simulador o dispositivo compatible
- **THEN** se muestra su pantalla inicial y se registra la plataforma, versión, resultado y evidencia visual

#### Scenario: Inicio bloqueado o fallido
- **WHEN** falta un runtime, dispositivo o herramienta, o la aplicación no abre
- **THEN** se registra el bloqueo o fallo y la plataforma no se marca como validada

### Requirement: Legibilidad y adaptación
La validación SHALL comprobar que el contenido disponible se puede leer y no queda recortado o cubierto por elementos del sistema en las configuraciones probadas.

#### Scenario: Cambios de presentación
- **WHEN** se revisan los tamaños de pantalla y modos previstos en el plan, incluidos texto ampliado en móviles
- **THEN** se documentan resultados y defectos de legibilidad, recorte, foco o áreas seguras cuando correspondan

### Requirement: Evidencia y límites
La validación SHALL distinguir compilación, pruebas automatizadas y revisión visual, y SHALL registrar solo resultados observados.

#### Scenario: Resultado registrado
- **WHEN** se termina una comprobación
- **THEN** se conservan comandos, versiones, fecha, capturas pertinentes y estado, sin incluir secretos

#### Scenario: Una plataforma pendiente
- **WHEN** alguna plataforma no puede ejecutarse o presenta fallos sin resolver
- **THEN** el cambio permanece abierto y el motivo queda documentado
