<!--
SYNC IMPACT REPORT
==================
Version change: (none) → 1.0.0  [INITIAL RATIFICATION]
Bump rationale: First adoption of the project constitution. MAJOR baseline.

Principles defined (7):
  I.   Confidencialidad del Token (NO NEGOCIABLE)
  II.  Propose / Confirm para Gasto Real (NO NEGOCIABLE)
  III. El Server es la Única Frontera con Meta (NO NEGOCIABLE)
  IV.  Auditoría de Toda Escritura
  V.   Errores Semánticos, Nunca Silenciados
  VI.  Rate Limiting Obligatorio
  VII. Mensajes Legibles para Usuario No Técnico

Sections added:
  - Restricciones Técnicas y Stack (cubre el stack obligatorio: Go, mcp-go, hexagonal, Railway)
  - Flujo de Desarrollo y Quality Gates
  - Governance

Sections removed: none (initial creation)

Templates reviewed for alignment:
  ✅ .specify/templates/plan-template.md     — "Constitution Check" gate is generic; principles map onto it. No edit required.
  ✅ .specify/templates/spec-template.md     — uses MUST-style FRs and tech-agnostic success criteria; compatible. No edit required.
  ✅ .specify/templates/tasks-template.md    — already includes logging, error-handling, security-hardening task slots that satisfy
                                               Principles IV/V/VII. No edit required.
  ✅ .specify/templates/checklist-template.md — generic; no constitution coupling.

Deferred / follow-up TODOs: none. RATIFICATION_DATE set to today (project initialized this session).
-->

# Meta Ads Manager Constitution

Servidor MCP en Go para la gestión de campañas de Meta Ads. Esta constitución define las
reglas no negociables que todo cambio, plan y revisión DEBE respetar. Donde aparece "DEBE",
la regla es de cumplimiento obligatorio; el incumplimiento bloquea el merge.

## Core Principles

### I. Confidencialidad del Token (NO NEGOCIABLE)

El token de acceso de Meta es el secreto de mayor criticidad del sistema.

- El token DEBE residir exclusivamente en variables de entorno del servidor.
- El token NUNCA DEBE aparecer en el repositorio (código, fixtures, tests, archivos `.env`
  versionados, ni configuración de Railway commiteada).
- El token NUNCA DEBE escribirse en logs, mensajes de error, trazas, respuestas MCP, ni
  telemetría. Todo log que pudiera contener credenciales DEBE redactarlas antes de emitir.
- La ausencia del token en el entorno DEBE provocar un fallo de arranque explícito (fail-fast),
  nunca un arranque degradado silencioso.

**Rationale**: Una sola fuga de token concede control total del gasto publicitario de la
cuenta a un tercero. La superficie de exposición se minimiza confinando el secreto a un único
lugar y prohibiendo su tránsito por cualquier canal observable.

### II. Propose / Confirm para Gasto Real (NO NEGOCIABLE)

Toda operación que afecte gasto real es irreversible y se ejecuta en dos pasos separados:

- **propose**: operación de solo lectura, sin efecto sobre Meta. Calcula y devuelve el cambio
  exacto que se aplicaría (qué, cuánto, sobre qué entidad).
- **confirm**: operación de escritura, irreversible, que aplica el cambio previamente propuesto.
- El paso `confirm` NUNCA DEBE ejecutarse sin un `propose` previo correspondiente. Claude NUNCA
  DEBE poder saltarse `propose`; el server DEBE rechazar cualquier `confirm` que no referencie
  una propuesta válida y vigente.

**Rationale**: El usuario final no técnico no puede evaluar el riesgo de una escritura
irreversible sobre presupuesto real. La separación obliga a una revisión explícita del efecto
antes de comprometer dinero, y hace estructuralmente imposible la escritura accidental.

### III. El Server es la Única Frontera con Meta (NO NEGOCIABLE)

El servidor MCP es el único componente que habla con la Meta Graph API.

- Claude (y cualquier cliente MCP) NUNCA DEBE tener acceso directo al token ni a la Graph API.
- Toda interacción con Meta DEBE pasar por las herramientas (tools) que el server expone.
- La capa de adaptador que llama a Meta DEBE estar aislada tras un puerto del dominio
  (ver arquitectura hexagonal), de modo que el resto del sistema no conozca al cliente HTTP de Meta.

**Rationale**: Confinar la frontera externa a un único punto concentra el control de
autorización, rate limiting, auditoría y redacción de secretos. Si Claude pudiera llamar a Meta
directamente, ninguno de los otros principios sería verificable.

### IV. Auditoría de Toda Escritura

Toda acción de escritura DEBE quedar registrada en un log estructurado.

- Cada registro DEBE incluir, como mínimo: timestamp, la entidad afectada, qué cambió
  (valor anterior → nuevo cuando aplique), y quién confirmó la acción.
- El log de auditoría DEBE ser estructurado (no texto libre) para permitir consulta y trazabilidad.
- La redacción de secretos del Principio I aplica también a estos registros.

**Rationale**: Sobre gasto real irreversible, la trazabilidad es un requisito de rendición de
cuentas, no una mejora opcional. Permite reconstruir qué se cambió y por qué ante una disputa.

### V. Errores Semánticos, Nunca Silenciados

Ningún fallo se silencia jamás.

- Si la Meta Graph API devuelve un error, el server DEBE devolver el error semántico correcto
  que corresponda a esa condición; NUNCA DEBE traducir un fallo en un éxito ni descartarlo.
- Los errores DEBEN propagarse explícitamente en cada capa; está prohibido el manejo que
  traga errores (catch vacío, error ignorado, fallback que oculta la causa).
- La distinción entre errores recuperables y no recuperables DEBE preservarse hasta el borde.

**Rationale**: Un fallo silenciado sobre operaciones de gasto produce divergencia invisible
entre el estado percibido y el estado real de la cuenta. La corrección del sistema depende de
que cada fallo sea observable y atribuible a su causa.

### VI. Rate Limiting Obligatorio

El respeto a los límites de la Meta API es obligatorio.

- El server DEBE respetar los límites de tasa de la Meta Graph API y NUNCA DEBE hacer hammering.
- El control de tasa DEBE vivir en el server (frontera única, Principio III), no delegarse al cliente.
- Ante señales de throttling de Meta, el server DEBE aplicar backoff en lugar de reintentar de forma agresiva.

**Rationale**: El hammering degrada el servicio para toda la cuenta y puede provocar bloqueos
temporales por parte de Meta, afectando a campañas en producción. El throttling responsable es
una condición de operación, no una optimización.

### VII. Mensajes Legibles para Usuario No Técnico

El usuario final no es técnico.

- Todo mensaje de error orientado al usuario DEBE ser legible en español y describir el problema
  en términos del dominio (campaña, presupuesto, anuncio), no en jerga técnica.
- El sistema NUNCA DEBE exponer stack traces, errores HTTP crudos ni detalles internos al usuario final.
- El contexto técnico detallado DEBE registrarse del lado del servidor (logs), separado del
  mensaje que ve el usuario.

**Rationale**: Un mensaje incomprensible sobre una operación de gasto erosiona la confianza y
puede inducir decisiones erróneas. La claridad en la frontera con el usuario es parte del contrato.

## Restricciones Técnicas y Stack

El stack siguiente es obligatorio salvo enmienda formal de esta constitución:

- **Lenguaje**: Go.
- **SDK MCP**: mcp-go.
- **Arquitectura**: hexagonal (ports & adapters). El dominio NO DEBE depender de detalles de
  infraestructura; la Meta Graph API, el transporte MCP y la persistencia se integran como
  adaptadores tras puertos del dominio. Esta separación es la que hace cumplibles los Principios III y V.
- **Deploy**: Railway.

Las decisiones que introduzcan dependencias o capas adicionales DEBEN justificarse frente a KISS
y YAGNI y registrarse en la sección "Complexity Tracking" del plan correspondiente.

## Flujo de Desarrollo y Quality Gates

- **Constitution Check**: todo plan (`plan.md`) DEBE pasar el gate de constitución antes de la
  fase de diseño y re-verificarse tras el diseño. Las violaciones DEBEN resolverse o justificarse
  explícitamente en "Complexity Tracking".
- **Pruebas**: las operaciones de escritura (Principio II) y el manejo de errores de Meta
  (Principio V) DEBEN cubrirse con tests antes de marcarse como completas.
- **Revisión de seguridad**: cualquier cambio que toque el manejo del token, la frontera con Meta,
  o el log de auditoría DEBE pasar una revisión de seguridad antes del merge (Principios I, III, IV).
- **Secretos**: ningún commit DEBE contener secretos; esto se verifica antes de cada merge.

## Governance

Esta constitución supersede cualquier otra práctica o convención del proyecto en caso de conflicto.

- **Enmiendas**: toda modificación DEBE documentarse en este archivo, justificar el cambio de
  versión y propagarse a las plantillas dependientes (`plan-template.md`, `spec-template.md`,
  `tasks-template.md`) mediante el Sync Impact Report.
- **Versionado semántico de la constitución**:
  - MAJOR: eliminación o redefinición incompatible de un principio o de la governance.
  - MINOR: adición de un principio/sección o expansión material de una guía existente.
  - PATCH: aclaraciones, correcciones de redacción o refinamientos no semánticos.
- **Cumplimiento**: todo PR y toda revisión DEBEN verificar el cumplimiento de los principios
  aplicables. Los principios marcados "NO NEGOCIABLE" bloquean el merge ante cualquier violación.
- **Revisión periódica**: el cumplimiento se evalúa en cada plan vía el Constitution Check; las
  desviaciones recurrentes DEBEN motivar una enmienda explícita en lugar de tolerarse de facto.

**Version**: 1.0.0 | **Ratified**: 2026-06-23 | **Last Amended**: 2026-06-23
