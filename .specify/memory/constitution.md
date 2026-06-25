<!--
SYNC IMPACT REPORT
==================
Version change: 1.0.0 → 1.1.0  [MINOR — adiciones, sin romper principios existentes]
Bump rationale: Se incorpora el contexto de negocio y dos principios nuevos (métricas con
  benchmarks; honestidad ante datos insuficientes). No se elimina ni redefine ningún principio
  previo, por lo que el cambio es MINOR según la política de versionado de la governance.

Principles (9 — 7 preexistentes sin cambios + 2 nuevos):
  I.    Confidencialidad del Token (NO NEGOCIABLE)            [sin cambios]
  II.   Propose / Confirm para Gasto Real (NO NEGOCIABLE)     [sin cambios]
  III.  El Server es la Única Frontera con Meta (NO NEGOCIABLE) [sin cambios]
  IV.   Auditoría de Toda Escritura                           [sin cambios]
  V.    Errores Semánticos, Nunca Silenciados                 [sin cambios]
  VI.   Rate Limiting Obligatorio                             [sin cambios]
  VII.  Mensajes Legibles para Usuario No Técnico             [sin cambios]
  VIII. Métricas y Umbrales del Negocio                       [NUEVO]
  IX.   Honestidad ante Datos Insuficientes (NO NEGOCIABLE)   [NUEVO]

Sections added:
  - Contexto de Negocio (rubro, mercado, presupuesto): define el dominio sobre el que operan
    los principios VII, VIII y IX.

Sections changed:
  - Restricciones Técnicas y Stack: sin cambios de contenido (Go, mcp-go, hexagonal, Railway).
  - Flujo de Desarrollo y Quality Gates: se añade el gate de benchmarks de métricas (Principio VIII).

Sections removed: none

Templates reviewed for alignment:
  ✅ .specify/templates/plan-template.md     — "Constitution Check" es genérico ("Gates determined
                                               based on constitution file"); los principios nuevos
                                               mapean sin editar la plantilla.
  ✅ .specify/templates/spec-template.md     — FRs MUST-style y criterios de éxito tech-agnósticos;
                                               compatible con benchmarks de negocio. Sin edición.
  ✅ .specify/templates/tasks-template.md    — slots de logging/errores/seguridad cubren IV/V/VII;
                                               las métricas se modelan como criterios de aceptación. Sin edición.
  ✅ .specify/templates/checklist-template.md — genérico; sin acople a la constitución.

Deferred / follow-up TODOs: none.
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

### VIII. Métricas y Umbrales del Negocio

El análisis de rendimiento DEBE interpretarse contra los umbrales de este negocio, no en
abstracto. Toda lectura, alerta o recomendación que involucre rendimiento DEBE evaluarse
con estos benchmarks de referencia:

- **ROAS** (retorno sobre inversión publicitaria): el mínimo aceptable es **2x**. Por debajo
  de 2x, el análisis DEBE señalarlo explícitamente como bajo rendimiento.
- **CPA** (costo por adquisición): DEBE evaluarse siempre **contra el ticket promedio** del
  negocio; un CPA nunca se reporta como "bueno" o "malo" en aislamiento, sino en relación al
  valor de la venta.
- **CTR** (click-through rate): el rango de referencia saludable es **0,8 % – 1,5 %**. Valores
  fuera de ese rango DEBEN destacarse (por debajo: creatividad/segmentación; muy por encima:
  posible tráfico de baja calidad).
- **Frecuencia**: DEBE emitirse una alerta cuando supere **3,5** (riesgo de fatiga de audiencia).

Estos umbrales son la referencia por defecto; cuando un análisis use otro umbral DEBE
justificarlo de forma explícita. Los valores DEBEN poder ajustarse por configuración sin
reescribir lógica, pero los defaults son los aquí definidos.

**Rationale**: Con un presupuesto acotado (ver Contexto de Negocio), cada peso mal asignado
pesa. Fijar umbrales explícitos convierte el análisis en accionable para un usuario no técnico
y evita interpretaciones arbitrarias de las mismas cifras.

### IX. Honestidad ante Datos Insuficientes (NO NEGOCIABLE)

El sistema NUNCA DEBE inventar conclusiones a partir de datos insuficientes.

- Cuando el volumen de datos (impresiones, clics, conversiones, días del período) sea demasiado
  bajo para sostener una conclusión, el sistema DEBE decirlo explícitamente en lugar de afirmar
  una tendencia o recomendación.
- Está prohibido presentar como certeza lo que es ruido estadístico; una métrica calculada sobre
  una muestra ínfima DEBE acompañarse de la advertencia correspondiente.
- Ante ausencia de datos para el período o la entidad pedida, la respuesta DEBE indicar que no
  hay datos suficientes, no devolver un resultado vacío que parezca una conclusión.

**Rationale**: Sobre decisiones de gasto real, una conclusión falsamente confiada es peor que
admitir incertidumbre: induce a reasignar presupuesto sobre evidencia inexistente. La honestidad
sobre los límites de los datos es condición de confianza con un usuario no técnico.

## Contexto de Negocio

Estos principios operan sobre un negocio concreto; las decisiones de diseño DEBEN tenerlo presente:

- **Rubro**: e-commerce de indumentaria, específicamente **sombreros de fieltro, sombreros de
  verano, boinas de mujer y boinas de hombre**. Es un negocio con estacionalidad marcada
  (fieltro/invierno vs. verano), lo que el análisis DEBE poder reflejar.
- **Mercado**: **Argentina** (moneda de la cuenta: ARS; mensajes y fechas en convención local).
- **Presupuesto publicitario**: **menor a USD 500/mes**. Es un presupuesto acotado: las
  recomendaciones DEBEN ser realistas para esa escala y el costo de error es alto (Principio VIII).

**Rationale**: Sin el contexto del negocio, las métricas y los mensajes serían genéricos. Anclar
el rubro, el mercado y el presupuesto hace que los umbrales (Principio VIII) y la honestidad de
datos (Principio IX) sean significativos en lugar de abstractos.

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
- **Benchmarks de métricas**: toda feature que lea o interprete rendimiento DEBE aplicar los
  umbrales del Principio VIII (ROAS, CPA vs. ticket, CTR, frecuencia) y respetar el Principio IX
  ante muestras insuficientes; los tests DEBEN cubrir los casos de umbral y de datos insuficientes.

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

**Version**: 1.1.0 | **Ratified**: 2026-06-23 | **Last Amended**: 2026-06-24
