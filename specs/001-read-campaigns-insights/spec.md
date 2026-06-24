# Feature Specification: Lectura de Campañas e Insights

**Feature Branch**: `001-read-campaigns-insights`

**Created**: 2026-06-23

**Status**: Draft

**Input**: User description: "Quiero primero arrancar por la feature get_campaigns + get_campaigns_insights. Sin poder leer información no puedo hacer mucho. Vamos a crear la estructura del proyecto, principalmente estos dos endpoints, para empezar a visualizar datos y tomar decisiones. El token de Meta ya está en el .env. Armar la estructura prolija pero sin sobreingeniería, para fallar rápido."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ver mis campañas (Priority: P1)

Como anunciante no técnico, le pido en lenguaje natural al asistente "mostrame mis campañas"
y obtengo la lista de campañas de mi cuenta publicitaria, cada una con su nombre, estado
(activa, pausada, finalizada) y objetivo. Esto me da el punto de partida para saber qué hay
y sobre qué entidad podría querer profundizar después.

**Why this priority**: Es la base de todo. Sin la lista de campañas no sé qué existe ni puedo
referirme a una campaña concreta para pedir su rendimiento. Es la primera unidad de valor
entregable y constituye el MVP del proyecto.

**Independent Test**: Con una cuenta que tenga al menos una campaña, pedir la lista y verificar
que devuelve cada campaña con nombre, estado y objetivo legibles. Entrega valor por sí sola:
el usuario ya puede ver el inventario de campañas aunque no exista nada más.

**Acceptance Scenarios**:

1. **Given** una cuenta con varias campañas, **When** el usuario pide ver sus campañas, **Then** recibe la lista completa con nombre, estado y objetivo de cada una.
2. **Given** una cuenta sin campañas, **When** el usuario pide ver sus campañas, **Then** recibe un mensaje claro en español indicando que no hay campañas, sin error técnico.
3. **Given** una credencial inválida o sin permisos, **When** el usuario pide ver sus campañas, **Then** recibe un mensaje en español que explica el problema en términos del dominio, sin stack trace ni detalles internos.

---

### User Story 2 - Ver el rendimiento de mis campañas (Priority: P2)

Como anunciante no técnico, pido el rendimiento de mis campañas (todas, o una en particular)
y obtengo las métricas clave —gasto, impresiones, clics, alcance y las tasas derivadas— para un
período de tiempo, de modo que pueda comparar y empezar a tomar decisiones.

**Why this priority**: Es la razón de fondo del proyecto ("tomar decisiones"), pero depende
funcionalmente de poder identificar campañas primero (US1). Se entrega inmediatamente después
del MVP y multiplica su valor.

**Independent Test**: Para una campaña conocida, pedir su rendimiento en un período y verificar
que se devuelven las métricas clave con valores y unidades legibles. Entrega valor por sí sola:
el usuario obtiene una foto cuantitativa del desempeño.

**Acceptance Scenarios**:

1. **Given** una campaña con actividad en el período, **When** el usuario pide su rendimiento, **Then** recibe gasto, impresiones, clics, alcance y tasas derivadas, con el período claramente indicado.
2. **Given** el usuario no especifica período, **When** pide el rendimiento, **Then** el sistema usa un período por defecto (últimos 30 días) y lo indica explícitamente en la respuesta.
3. **Given** una campaña sin actividad en el período, **When** el usuario pide su rendimiento, **Then** recibe un mensaje claro de que no hubo actividad, sin confundirlo con un error.
4. **Given** la fuente de datos responde con error, **When** el usuario pide el rendimiento, **Then** recibe un mensaje semántico en español y el fallo queda registrado del lado del servidor; el sistema nunca devuelve un éxito falso.

---

### Edge Cases

- **Sin resultados vs. error**: la ausencia de campañas o de actividad NO es un error y debe comunicarse de forma distinta a un fallo de la fuente de datos.
- **Credencial ausente al iniciar**: si la credencial de la cuenta no está disponible, el sistema falla de forma explícita al arrancar, no en mitad de una consulta.
- **Límite de uso de la fuente de datos**: ante señales de saturación, el sistema respeta el límite y espera en lugar de insistir agresivamente; el usuario recibe, si corresponde, un mensaje de "intentá de nuevo en unos momentos".
- **Volumen alto de campañas**: una cuenta con muchas campañas debe poder listarse sin que la respuesta se vuelva inmanejable (paginación o límite con indicación de que hay más).
- **Período inválido**: si el usuario pide un rango de fechas imposible (fin antes de inicio, futuro), recibe un mensaje claro pidiendo corregirlo.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema MUST exponer una operación de solo lectura que devuelva la lista de campañas de la cuenta publicitaria configurada, incluyendo, por cada campaña, un identificador, su nombre, su estado y su objetivo.
- **FR-002**: El sistema MUST exponer una operación de solo lectura que devuelva métricas de rendimiento de campañas para un período: gasto, impresiones, clics, alcance y las tasas derivadas habituales (p. ej. CTR y costo por clic).
- **FR-003**: La operación de rendimiento MUST permitir consultar todas las campañas o una campaña específica identificada por el usuario.
- **FR-004**: Cuando el usuario no especifica un período, el sistema MUST aplicar un período por defecto (últimos 30 días) e indicarlo explícitamente en la respuesta.
- **FR-005**: Ninguna de estas operaciones MUST producir efecto alguno sobre el gasto ni sobre el estado de las campañas; son estrictamente de lectura.
- **FR-006**: El sistema MUST distinguir y comunicar de forma diferenciada tres situaciones: resultado vacío (sin campañas / sin actividad), error recuperable (p. ej. saturación temporal) y error no recuperable (p. ej. credencial inválida).
- **FR-007**: Todo mensaje dirigido al usuario MUST estar redactado en español y en términos del dominio (campaña, gasto, anuncio), sin exponer stack traces, códigos crudos ni detalles internos.
- **FR-008**: El sistema MUST registrar del lado del servidor el contexto técnico de cualquier fallo, separado del mensaje que ve el usuario, y MUST NOT silenciar fallos devolviendo un éxito falso.
- **FR-009**: La credencial de acceso a la cuenta MUST obtenerse exclusivamente de la configuración del entorno del servidor y MUST NOT aparecer en respuestas, mensajes ni registros.
- **FR-010**: Si la credencial requerida no está disponible al iniciar, el sistema MUST fallar de forma explícita en el arranque con un mensaje claro, en lugar de arrancar en estado degradado.
- **FR-011**: El sistema MUST respetar los límites de uso de la fuente de datos, aplicando espera/backoff ante señales de saturación en vez de reintentar agresivamente.
- **FR-012**: Ante un volumen alto de campañas, el sistema MUST acotar la respuesta (límite o paginación) e indicar al usuario cuando existan más resultados de los devueltos.

### Key Entities *(include if feature involves data)*

- **Campaña**: unidad publicitaria de la cuenta. Atributos relevantes para esta feature: identificador, nombre, estado (activa / pausada / finalizada), objetivo.
- **Rendimiento de campaña (insight)**: conjunto de métricas asociadas a una campaña durante un período. Atributos: gasto, impresiones, clics, alcance, tasas derivadas (CTR, costo por clic), y el período al que corresponden.
- **Cuenta publicitaria**: la cuenta a la que pertenecen las campañas; está determinada por la configuración del entorno del servidor.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un usuario no técnico puede obtener la lista de sus campañas con un único pedido en lenguaje natural, sin conocer identificadores previamente.
- **SC-002**: Un usuario puede obtener el rendimiento de una campaña para un período en un único pedido, viendo el período aplicado de forma explícita.
- **SC-003**: El 100% de los mensajes de error que ve el usuario están en español y no contienen stack traces ni detalles técnicos.
- **SC-004**: En el 100% de los casos en que la fuente de datos falla, el usuario recibe un mensaje semántico y el sistema nunca reporta éxito falso.
- **SC-005**: Las situaciones "sin campañas" y "sin actividad" se comunican como estados normales, distintos de un error, en el 100% de los casos.
- **SC-006**: Las consultas de lectura no modifican en ningún caso el estado ni el gasto de las campañas (verificable: cero operaciones de escritura emitidas).
- **SC-007**: La credencial de acceso no aparece en ninguna respuesta ni registro (verificable mediante inspección de salidas y logs).

## Assumptions

- **Cuenta única**: existe una sola cuenta publicitaria configurada por entorno; el soporte multicuenta queda fuera de alcance en esta primera versión.
- **Período por defecto**: a falta de indicación del usuario, se usan los últimos 30 días; el usuario puede especificar otro rango.
- **Métricas iniciales**: el conjunto de métricas de rendimiento se limita a gasto, impresiones, clics, alcance y tasas derivadas (CTR, costo por clic); métricas de conversión avanzadas quedan para iteraciones posteriores.
- **Credencial provista**: el token de acceso ya está disponible en la configuración del entorno (`.env`) con los permisos de lectura necesarios; su gestión y rotación quedan fuera del alcance de esta feature.
- **Solo lectura**: esta feature no incluye ninguna operación de escritura; por tanto no requiere el flujo propose/confirm de la constitución, que aplicará a futuras features de escritura.
- **Idioma**: la audiencia final es hispanohablante; todos los mensajes orientados al usuario se entregan en español.
- **Estructura inicial**: se establece la estructura base del proyecto en esta feature, priorizando simplicidad y "fallar rápido" por sobre generalización anticipada.
