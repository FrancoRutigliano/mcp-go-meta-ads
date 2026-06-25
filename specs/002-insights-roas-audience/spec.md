# Feature Specification: Insights de Decisión (ROAS/CPA + Desglose de Audiencias)

**Feature Branch**: `002-insights-roas-audience`

**Created**: 2026-06-24

**Status**: Draft

**Input**: Brief: extender la lectura de rendimiento con métricas de conversión (ROAS, compras, facturación, CPA, CTR de enlace, frecuencia) evaluadas contra los umbrales de la constitución, y agregar el desglose por audiencia (edad, género, región, plataforma, posición). Todo SOLO LECTURA: informa y recomienda, no apaga ni prende nada.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Evaluar una campaña para decidir (Priority: P1)

Como anunciante no técnico, pido el rendimiento de una campaña (o de todas) y, además del
gasto y los clics, obtengo **ROAS, cantidad de compras, facturación atribuida, CPA, CTR de
enlace y frecuencia**, cada métrica clave acompañada de un estado visual (cumple / atención /
no cumple) según los umbrales del negocio. Así puedo responder "¿esta campaña conviene
apagarla o dejarla?" leyendo, sin tocar nada.

**Why this priority**: Es el corazón de la feature. Sin ROAS/CPA/conversiones no se puede
evaluar una campaña de Ventas. Entrega por sí sola la capacidad de decisión sobre campañas y
es el MVP de esta entrega.

**Independent Test**: Pedir el rendimiento de una campaña con conversiones en un período y
verificar que devuelve las métricas nuevas, cada una marcada según su umbral, con el período
indicado. Entrega valor solo: el usuario ya puede juzgar la campaña.

**Acceptance Scenarios**:

1. **Given** una campaña de Ventas con compras en el período, **When** pido su rendimiento, **Then** recibo ROAS, nº de compras, facturación, CPA, CTR de enlace y frecuencia, además del gasto/impresiones/clics/alcance ya existentes, con el período aplicado indicado.
2. **Given** una campaña con ROAS por debajo del umbral (2x), **When** pido su rendimiento, **Then** el ROAS aparece marcado como "no cumple" (candidata a apagar).
3. **Given** una campaña con CTR de enlace fuera del rango 0,8–1,5%, **When** pido su rendimiento, **Then** el CTR de enlace aparece marcado como "atención".
4. **Given** una campaña con frecuencia mayor a 3,5, **When** pido su rendimiento, **Then** aparece una alerta de fatiga de audiencia.
5. **Given** una campaña con CPA por encima del ticket promedio, **When** pido su rendimiento, **Then** el CPA aparece marcado como "no cumple / revisar".

---

### User Story 2 - Desglosar por audiencia para reasignar (Priority: P2)

Como anunciante no técnico, pido el rendimiento segmentado por una dimensión —edad, género,
región, plataforma o posición del anuncio— para ver qué público o ubicación rinde mejor y
decidir hacia dónde convendría reasignar. Puedo pedirlo para una campaña puntual o para toda
la cuenta.

**Why this priority**: Multiplica el valor de US1 al permitir decisiones más finas (qué
público apagar/reforzar), pero depende de tener primero las métricas de decisión definidas.

**Independent Test**: Pedir el desglose de una campaña por una dimensión válida (p. ej.
"edad") en un período y verificar que devuelve las mismas métricas de decisión por cada
segmento, en español. Entrega valor solo: el usuario ve qué segmento rinde mejor.

**Acceptance Scenarios**:

1. **Given** una campaña con actividad, **When** pido el desglose por "edad", **Then** recibo cada franja etaria con sus métricas de decisión y estados vs umbral.
2. **Given** una dimensión soportada (edad, género, región, plataforma, posición), **When** la pido, **Then** el sistema la acepta y segmenta por ella.
3. **Given** una combinación de segmentación no soportada por la plataforma, **When** la pido, **Then** recibo un mensaje claro en español explicando que esa combinación no es válida, sin error técnico ni éxito falso.
4. **Given** no indico período, **When** pido un desglose, **Then** se usan los últimos 30 días y se indica explícitamente.

---

### User Story 3 - Respuesta honesta ante datos insuficientes (Priority: P1)

Como anunciante no técnico, cuando una campaña o segmento tiene muy pocos datos (poca
actividad, pocos días, o el píxel no registró compras), necesito que el sistema me lo diga
explícitamente en lugar de mostrarme un ROAS de 0 o un CPA como si fueran conclusiones
reales, para no tomar una decisión equivocada.

**Why this priority**: Es crítico (Principio IX, no negociable). Mostrar "ROAS 0" sobre una
muestra ínfima induce a apagar una campaña que quizá funciona. Debe entregarse junto con US1.

**Independent Test**: Pedir el rendimiento de una campaña sin compras registradas (o con
muestra mínima) y verificar que la respuesta dice "datos insuficientes / no se puede calcular
ROAS/CPA" en vez de mostrar ceros como resultado.

**Acceptance Scenarios**:

1. **Given** una campaña sin compras atribuidas en el período, **When** pido su rendimiento, **Then** ROAS y CPA se reportan como "no calculable por falta de conversiones", no como 0.
2. **Given** una campaña con muestra ínfima (muy pocas impresiones/clics o muy pocos días), **When** pido su rendimiento, **Then** las métricas que dependen de esa muestra se marcan como "datos insuficientes" y no se emite una recomendación de apagar/prender.
3. **Given** un segmento sin actividad en un desglose, **When** lo recibo, **Then** se muestra como "sin datos", distinto de un segmento con bajo rendimiento real.

---

### Edge Cases

- **Sin conversiones vs. ROAS 0**: la ausencia de compras NO es ROAS 0; debe comunicarse como "no calculable".
- **Métrica clave faltante**: si la plataforma no devuelve una métrica concreta (p. ej. facturación), esa métrica se marca como no disponible, sin inventar valores.
- **CTR total vs CTR de enlace**: la evaluación del umbral 0,8–1,5% usa el CTR de enlace, no el CTR de todos los clics; ambos pueden mostrarse pero el umbral aplica al de enlace.
- **Combinación de segmentación inválida**: ciertas dimensiones no se combinan con ciertas métricas; el caso se informa como pedido inválido, no como fallo del sistema.
- **Período inválido**: rango invertido o futuro → mensaje claro pidiendo corregir (igual que en la feature de lectura existente).
- **Ticket promedio no derivable**: si no hay facturación ni compras y no hay override configurado, el CPA no se compara contra ticket y se indica que falta la referencia.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La consulta de rendimiento de campañas MUST devolver, además de las métricas actuales (gasto, impresiones, clics, alcance), las métricas de conversión: **ROAS, cantidad de compras, facturación atribuida, CPA, CTR de enlace y frecuencia**, con el período aplicado indicado.
- **FR-002**: El sistema MUST evaluar cada métrica clave contra los umbrales del negocio y devolver su estado: **cumple / atención / no cumple / sin datos**. Umbrales por defecto: ROAS ≥ 2x; CTR de enlace entre 0,8% y 1,5%; frecuencia ≤ 3,5; CPA comparado contra el ticket promedio.
- **FR-003**: El sistema MUST usar el CTR **de enlace** para evaluar el umbral 0,8–1,5% (no el CTR de todos los clics), de modo que la comparación sea correcta.
- **FR-004**: El ticket promedio usado para evaluar el CPA MUST derivarse de facturación ÷ cantidad de compras cuando haya datos suficientes, y MUST poder fijarse mediante configuración del negocio cuando se quiera un valor explícito.
- **FR-005**: Los umbrales (ROAS, CTR de enlace, frecuencia, ticket) MUST ser ajustables por configuración, con los valores anteriores como defaults.
- **FR-006**: El sistema MUST ofrecer una consulta de **desglose por audiencia** que devuelva las mismas métricas de decisión segmentadas por una dimensión, soportando: **edad, género, región, plataforma y posición del anuncio**.
- **FR-007**: La consulta de desglose MUST permitir limitarse a una campaña específica o, si no se indica, operar a nivel de la cuenta.
- **FR-008**: Si no se indica período, ambas consultas MUST aplicar los últimos 30 días e indicarlo explícitamente.
- **FR-009**: Cuando no hay compras atribuidas, el sistema MUST reportar ROAS y CPA como **"no calculable por falta de conversiones"**, y NUNCA mostrar 0 como si fuera un resultado real.
- **FR-010**: Cuando la muestra es insuficiente (actividad o período mínimos), el sistema MUST marcar las métricas afectadas como **"datos insuficientes"** y NO emitir una recomendación de apagar/prender sobre esa base.
- **FR-011**: Ante una combinación de segmentación no soportada, el sistema MUST devolver un mensaje claro en español indicando que el pedido no es válido, sin exponer detalles técnicos y sin reportar éxito falso.
- **FR-012**: Todos los mensajes y estados orientados al usuario MUST estar en español, en términos del dominio, sin stack traces ni jerga técnica.
- **FR-013**: Esta feature MUST ser estrictamente de **solo lectura**: no apaga, prende ni modifica campañas, presupuestos ni audiencias (cero operaciones de escritura).
- **FR-014**: El sistema MUST distinguir tres situaciones y comunicarlas de forma diferenciada: resultado con datos, resultado vacío/sin actividad, y error (recuperable o no), sin confundirlas.

### Key Entities *(include if feature involves data)*

- **Rendimiento de campaña (extendido)**: además de gasto/impresiones/clics/alcance, incorpora ROAS, compras, facturación, CPA, CTR de enlace y frecuencia, más el estado de cada métrica clave frente a su umbral.
- **Desglose de audiencia**: conjunto de segmentos de una dimensión (edad, género, región, plataforma, posición), cada uno con las métricas de decisión y sus estados.
- **Dimensión de segmentación**: la categoría por la que se segmenta (edad | género | región | plataforma | posición).
- **Estado de métrica**: clasificación de cada métrica clave (cumple | atención | no cumple | sin datos / no calculable).
- **Umbrales del negocio**: ROAS objetivo, rango de CTR de enlace, tope de frecuencia y ticket promedio; con defaults y posibilidad de override por configuración.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Para una campaña con conversiones, el usuario obtiene en un único pedido ROAS, compras, facturación, CPA, CTR de enlace y frecuencia, con el período indicado.
- **SC-002**: El 100% de las métricas clave se muestran con su estado vs umbral (cumple / atención / no cumple / sin datos).
- **SC-003**: En el 100% de los casos sin compras atribuidas, ROAS y CPA se reportan como "no calculable", nunca como 0.
- **SC-004**: En el 100% de los casos de muestra insuficiente, el sistema lo declara explícitamente y no recomienda apagar/prender.
- **SC-005**: El desglose por audiencia devuelve las métricas de decisión por cada segmento de la dimensión pedida, en español.
- **SC-006**: Una combinación de segmentación inválida se comunica como pedido inválido en español en el 100% de los casos, sin éxito falso.
- **SC-007**: Cero operaciones de escritura emitidas hacia la plataforma (verificable por inspección).
- **SC-008**: La evaluación del umbral de CTR usa el CTR de enlace en el 100% de los casos.

## Assumptions

- **Umbrales por defecto (Principio VIII)**: ROAS ≥ 2x, CTR de enlace 0,8–1,5%, frecuencia ≤ 3,5, CPA vs ticket promedio; todos ajustables por configuración.
- **Definición de "compra"**: se consideran compras las acciones de tipo compra atribuidas por la plataforma (incluye variantes de compra por píxel y compra en sitio).
- **Datos insuficientes (Principio IX)**: se aplican defaults conservadores para considerar una muestra ínfima (p. ej. período muy corto, o muy pocas impresiones/clics/compras); estos límites son ajustables por configuración. Los valores concretos se fijan en la fase de planificación.
- **Ticket promedio**: el ticket promedio por compra del negocio es **≈ 80.000 ARS** (dato real de la tienda en Tienda Nube). Este valor se usa como **default de negocio** para comparar el CPA, y es ajustable por configuración. Cuando haya datos del período (facturación ÷ compras) puede derivarse el ticket real observado; si no hay datos ni override, se cae al default de 80.000 ARS y se indica que es una referencia estimada. Si tampoco hubiera referencia disponible, el CPA se muestra sin comparación e indicando la falta.
- **Período por defecto**: últimos 30 días, consistente con la feature de lectura existente.
- **Nivel de análisis**: esta entrega opera a nivel de campaña y de cuenta. El análisis a nivel de "público" concreto (que en la plataforma se define en una capa más granular que la campaña) queda como posible extensión a evaluar en planificación, fuera del alcance comprometido aquí.
- **Cuenta única**: se mantiene el supuesto de una sola cuenta configurada por entorno.

## Out of Scope

- Apagar/prender campañas o cambiar presupuestos: es una feature de **escritura** futura, sujeta al flujo propose/confirm de la constitución.
- Multicuenta.
- Crear o editar públicos/audiencias.
- Análisis a nivel de público granular (capa de conjunto de anuncios), salvo que se decida incorporarlo explícitamente en planificación.
