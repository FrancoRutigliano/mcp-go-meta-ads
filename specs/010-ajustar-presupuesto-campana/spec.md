# Feature Specification: Ajustar presupuesto de campañas

**Feature Branch**: `010-ajustar-presupuesto-campana`

**Created**: 2026-07-28

**Status**: Draft

**Input**: User description: "Spec detallada #010: poder cambiar presupuestos o ajustar estos para diferentes campañas."

## Contexto

El roadmap resume esta feature como *"poné la plata en lo que funciona"*. Hoy el sistema ya sabe
**qué funciona** (001–005: ROAS, embudo, públicos, rendimiento por anuncio) y ya sabe **escribir de
forma segura** (009: cimiento propose/confirm + auditoría, con pausar/activar campaña como primera
operación). Lo que falta es la acción que cierra el ciclo: **mover el dinero**.

Esta feature reutiliza el mismo cimiento de escritura de la 009. No introduce un segundo camino de
escritura: toda modificación de presupuesto pasa por `propose` → `confirm`, igual que el cambio de
estado.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Subir o bajar el presupuesto de una campaña (Priority: P1)

Mariana ve que la campaña de Ventas rinde 6,39x y quiere ponerle más plata. Le pide a Claude en
español: *"subile el presupuesto a la campaña de Ventas a 5.000 pesos por día"*. El sistema le
muestra exactamente qué va a cambiar —campaña, presupuesto actual, presupuesto propuesto, cuánto
representa el cambio— y **no toca nada** hasta que ella diga que sí. Recién cuando confirma, el
cambio se aplica en Meta y queda registrado en la auditoría.

**Why this priority**: Es la razón de ser de la feature. Sin esto, el resto no tiene valor. Entrega
por sí sola el bucle completo "veo que anda → le pongo plata".

**Independent Test**: Se puede probar de punta a punta pidiendo un cambio de presupuesto sobre una
campaña real, verificando que el paso de propuesta no modifica nada en Meta y que sólo la
confirmación aplica el cambio observable en la cuenta.

**Acceptance Scenarios**:

1. **Given** una campaña con presupuesto diario de $3.000, **When** el usuario propone subirlo a
   $5.000, **Then** el sistema devuelve una propuesta que describe campaña, valor anterior ($3.000),
   valor propuesto ($5.000) y la variación (+67 %), y el presupuesto en Meta sigue en $3.000.
2. **Given** una propuesta de presupuesto vigente, **When** el usuario la confirma, **Then** el
   presupuesto queda efectivamente cambiado en Meta y se registra en la auditoría qué campaña, qué
   valor antes, qué valor después, quién confirmó y cuándo.
3. **Given** una propuesta de presupuesto vigente, **When** el usuario **no** la confirma, **Then**
   el presupuesto en Meta permanece sin cambios y la propuesta vence sin efecto.
4. **Given** una propuesta ya confirmada, **When** se intenta confirmarla de nuevo, **Then** el
   sistema la rechaza y no vuelve a aplicar el cambio.
5. **Given** una campaña cuyo presupuesto actual ya es el valor pedido, **When** se propone ese
   mismo valor, **Then** el sistema rechaza la propuesta explicando que no hay cambio que aplicar.

---

### User Story 2 - Ajustar el presupuesto de un conjunto de anuncios (Priority: P2)

Muchas campañas no tienen el presupuesto en la campaña sino repartido en sus **conjuntos de
anuncios** (cada conjunto es un público con su propia plata). Mariana pide *"subile el presupuesto a
la campaña de Ventas"* y el sistema le responde que esa campaña reparte la plata entre 3 conjuntos,
le muestra cuánto tiene cada uno y cómo viene rindiendo, y le permite ajustar el que ella elija —con
la misma propuesta y confirmación de siempre.

**Why this priority**: Sin esto, la feature puede no aplicar a ninguna campaña real de la cuenta. Es
P2 y no P1 sólo porque el flujo de campaña define el mecanismo que después se reutiliza.

**Independent Test**: Se puede probar sobre una campaña que reparte presupuesto en conjuntos,
verificando que el sistema identifica dónde vive la plata, lista los conjuntos con su monto actual, y
aplica el cambio sólo al conjunto elegido y sólo tras confirmar.

**Acceptance Scenarios**:

1. **Given** una campaña cuyo presupuesto vive en sus conjuntos de anuncios, **When** el usuario
   propone cambiar el presupuesto *de la campaña*, **Then** el sistema no falla: explica que la plata
   está en los conjuntos y lista cada uno con su nombre y su presupuesto actual.
2. **Given** un conjunto de anuncios con presupuesto diario de $1.500, **When** el usuario propone
   subirlo a $2.500, **Then** el sistema devuelve una propuesta con conjunto, campaña a la que
   pertenece, valor anterior, valor propuesto y variación, sin tocar nada en Meta.
3. **Given** una propuesta sobre un conjunto de anuncios, **When** el usuario la confirma, **Then**
   el cambio se aplica sólo a ese conjunto, los demás conjuntos de la campaña quedan intactos, y la
   auditoría registra el conjunto afectado además de la campaña.
4. **Given** una campaña que sí administra su propio presupuesto, **When** el usuario intenta ajustar
   el presupuesto de uno de sus conjuntos, **Then** el sistema explica que en esa campaña el
   presupuesto se controla a nivel campaña.

---

### User Story 3 - Decidir con el rendimiento a la vista (Priority: P2)

Antes de mover plata, Mariana necesita ver si la campaña lo merece. Cuando pide un cambio de
presupuesto, la propuesta le muestra —junto al monto— el rendimiento reciente de esa campaña
(ROAS, gasto, compras del último período) y una lectura honesta: *"esta campaña rinde 6,39x, por
encima del mínimo de 2x"* o *"con 3 compras en 7 días no hay datos suficientes para sostener que
convenga subirle el presupuesto"*.

**Why this priority**: Convierte una operación mecánica en una decisión informada. Sin esto, el
usuario no técnico decide a ciegas sobre gasto irreversible. Es valioso pero la operación funciona
sin ello, por eso es P2.

**Independent Test**: Se puede probar pidiendo propuestas sobre una campaña con buen rendimiento y
sobre otra con volumen ínfimo, verificando que el texto de la propuesta refleja ROAS contra el
umbral de 2x en el primer caso y una advertencia de datos insuficientes en el segundo.

**Acceptance Scenarios**:

1. **Given** una campaña con ROAS 6,39x en el período de referencia, **When** se propone subirle el
   presupuesto, **Then** la propuesta incluye ese ROAS y lo señala como por encima del mínimo de 2x.
2. **Given** una campaña con ROAS 0,9x, **When** se propone subirle el presupuesto, **Then** la
   propuesta incluye una advertencia explícita de bajo rendimiento antes de que el usuario confirme.
3. **Given** una campaña sin conversiones o con volumen ínfimo en el período, **When** se propone un
   cambio de presupuesto, **Then** la propuesta declara que no hay datos suficientes en lugar de
   afirmar una recomendación, y aun así permite confirmar si el usuario insiste.

---

### User Story 4 - Reasignar presupuesto entre campañas (Priority: P3)

Mariana quiere *"sacarle a la que no anda y ponerle a la que anda"*. El sistema le permite hacerlo
como dos operaciones encadenadas —bajar el presupuesto de una, subir el de la otra— cada una con su
propia propuesta y su propia confirmación, mostrando el efecto neto sobre el gasto diario total.

**Why this priority**: Es el uso real más frecuente, pero se resuelve componiendo la P1 dos veces.
No requiere capacidad nueva del sistema, sólo que la P1 exista y que el efecto neto sea visible.

**Independent Test**: Se puede probar bajando el presupuesto de una campaña y subiendo el de otra en
secuencia, verificando que cada paso pide su confirmación por separado y que ninguna confirmación
arrastra a la otra.

**Acceptance Scenarios**:

1. **Given** dos campañas con presupuesto asignado, **When** el usuario baja el de una y sube el de
   la otra, **Then** cada cambio exige su propia confirmación y se audita por separado.
2. **Given** una reasignación en curso, **When** el usuario confirma sólo el primer cambio y abandona
   el segundo, **Then** el primero queda aplicado y el segundo no produce ningún efecto.

---

### Edge Cases

- **La campaña no administra su propio presupuesto**: en Meta, una campaña sin presupuesto a nivel
  campaña tiene el presupuesto repartido en sus conjuntos de anuncios. Intentar escribir un
  presupuesto de campaña ahí falla. El sistema DEBE detectarlo en el paso de propuesta, explicarlo
  en castellano y redirigir al nivel correcto, no dejar que la confirmación falle contra Meta.
- **El nivel equivocado en el otro sentido**: pedir un cambio sobre un conjunto de anuncios cuya
  campaña controla el presupuesto centralmente también debe rechazarse con la explicación inversa.
- **Campaña sin conjuntos de anuncios**: no hay dónde ajustar; el sistema debe decirlo en lugar de
  devolver una lista vacía que parezca un resultado.
- **Ajuste relativo sobre presupuesto inexistente**: pedir "subile 30 %" sobre una entidad sin
  presupuesto configurado no tiene base de cálculo y debe rechazarse explicando por qué.
- **Redondeo del ajuste relativo**: un porcentaje puede dar un monto con decimales; el resultado debe
  redondearse a una unidad de moneda razonable y mostrarse ya redondeado en la propuesta, para que lo
  que el usuario aprueba sea exactamente lo que se aplica.
- **Tipo de presupuesto distinto**: una campaña con presupuesto **total** (por toda la duración) no
  es lo mismo que una con presupuesto **diario**. Cambiar de un tipo al otro no es un ajuste de
  monto sino un cambio de configuración con efectos distintos sobre el gasto.
- **Monto por debajo del mínimo de Meta**: Meta impone un presupuesto diario mínimo según moneda y
  tipo de puja. Un monto por debajo debe rechazarse con un mensaje comprensible, no con el error
  crudo de la API.
- **Aumento desmedido**: pedir un salto de $3.000 a $300.000 por día sobre un presupuesto mensual
  menor a USD 500 es casi seguro un error de tipeo o de interpretación. El sistema debe frenarlo.
- **Presupuesto cero o negativo**: no es una forma válida de pausar una campaña; para eso existe la
  operación de pausar de la 009.
- **Campaña archivada o eliminada**: no admite cambios de presupuesto.
- **Campaña pausada**: admite cambio de presupuesto, pero el cambio no produce gasto hasta que se
  active; el sistema debe aclararlo para no dar una falsa sensación de efecto inmediato.
- **Propuesta vencida**: pasado el tiempo de vigencia, confirmar debe fallar y obligar a proponer de
  nuevo con el estado actual leído otra vez.
- **El presupuesto cambió entre la propuesta y la confirmación** (cambio hecho desde el Business
  Manager por otra persona): la confirmación estaría aplicando un cambio sobre una base que ya no es
  la que se le mostró al usuario.
- **Token sin permiso de escritura**: debe producir un mensaje que indique que falta el permiso para
  administrar anuncios, no un error de autorización crudo.
- **Campaña inexistente o ID mal escrito**: mensaje claro de campaña no encontrada.
- **Moneda**: los montos se expresan y se leen en pesos argentinos; el usuario nunca ve unidades
  internas de la API.

## Requirements *(mandatory)*

### Functional Requirements

**Propuesta (paso 1, sin efecto)**

- **FR-001**: El sistema DEBE permitir proponer un nuevo presupuesto para una campaña o para un
  conjunto de anuncios identificado, sin producir ningún cambio en Meta durante ese paso.
- **FR-002**: La propuesta DEBE leer el presupuesto actual real de la entidad en el momento de
  proponerse, y no asumirlo a partir de una lectura previa.
- **FR-003**: La propuesta DEBE informar: nivel afectado (campaña o conjunto), nombre de la entidad,
  campaña a la que pertenece cuando sea un conjunto, presupuesto actual, presupuesto propuesto, la
  variación absoluta y porcentual, y el tipo de presupuesto (diario o total).
- **FR-004**: La propuesta DEBE aclarar si el cambio produce gasto inmediato o no, según el estado
  de la entidad (activa vs. pausada).
- **FR-005**: El sistema DEBE rechazar la propuesta cuando el monto pedido sea igual al presupuesto
  actual, indicando que no hay cambio que aplicar.
- **FR-006**: El sistema DEBE rechazar montos cero, negativos o no numéricos con un mensaje del
  dominio, sin exponer detalles técnicos.

**Nivel del presupuesto (campaña vs. conjunto de anuncios)**

- **FR-007**: El sistema DEBE determinar, al proponer, en qué nivel vive realmente el presupuesto de
  la campaña, y DEBE rechazar la propuesta cuando se pida escribir en el nivel equivocado, en
  cualquiera de los dos sentidos.
- **FR-008**: Cuando se pida cambiar el presupuesto de una campaña que reparte la plata en sus
  conjuntos de anuncios, el sistema DEBE listar esos conjuntos con su nombre, estado y presupuesto
  actual, para que el usuario elija dónde aplicar el cambio sin salir de la conversación.
- **FR-009**: El sistema DEBE permitir consultar el presupuesto vigente de una campaña y de sus
  conjuntos sin proponer ningún cambio, como operación de sólo lectura.
- **FR-010**: Una campaña sin conjuntos de anuncios DEBE reportarse como tal, no como una lista vacía.

**Forma de expresar el cambio**

- **FR-011**: El sistema DEBE aceptar el nuevo presupuesto expresado como **monto absoluto** en pesos
  (por ejemplo, "$5.000 por día").
- **FR-012**: El sistema DEBE aceptar el cambio expresado como **ajuste relativo** sobre el
  presupuesto vigente (por ejemplo, "subir 30 %", "bajar a la mitad"), calculando el monto resultante
  del lado del servidor sobre el presupuesto leído en ese mismo momento.
- **FR-013**: Toda propuesta, sea absoluta o relativa, DEBE mostrar el **monto absoluto resultante**
  ya redondeado; el usuario nunca confirma un porcentaje sino la cifra final que se va a aplicar.
- **FR-014**: El sistema DEBE rechazar un ajuste relativo cuando la entidad no tenga un presupuesto
  vigente sobre el cual calcularlo.
- **FR-015**: El sistema DEBE rechazar la combinación simultánea de monto absoluto y ajuste relativo
  en una misma solicitud, en lugar de elegir uno por su cuenta.

**Guardrails de gasto**

- **FR-016**: El sistema DEBE rechazar cualquier propuesta que multiplique el presupuesto vigente por
  más de un factor máximo configurable (por defecto **3x** en una sola operación).
- **FR-017**: El sistema DEBE rechazar cualquier propuesta cuyo presupuesto diario resultante supere
  un techo absoluto configurable, cuyo valor por defecto DEBE ser coherente con un gasto mensual
  menor a USD 500.
- **FR-018**: Ambos límites DEBEN ser ajustables por configuración del servidor sin modificar la
  lógica, y el mensaje de rechazo DEBE indicar cuál de los dos se superó y cuál es el máximo
  admitido.
- **FR-019**: El sistema DEBE mostrar el presupuesto diario resultante y su proyección mensual
  aproximada, para que el usuario dimensione el compromiso frente a un presupuesto acotado.
- **FR-020**: El sistema NUNCA DEBE aplicar un cambio de presupuesto sin una propuesta previa válida
  y vigente que lo respalde (Principio II).
- **FR-021**: El sistema DEBE rechazar cambios de presupuesto sobre entidades archivadas o eliminadas.
- **FR-022**: Cada propuesta DEBE tener un identificador único y una vigencia limitada, vencida la
  cual ya no puede confirmarse.

**Contexto de rendimiento**

- **FR-023**: La propuesta DEBE incluir el rendimiento reciente de la entidad afectada (ROAS, gasto y
  compras del período de referencia) para que la decisión sea informada.
- **FR-024**: El sistema DEBE evaluar el ROAS contra el mínimo de 2x y señalar explícitamente el
  bajo rendimiento cuando esté por debajo (Principio VIII).
- **FR-025**: Cuando el volumen de datos sea insuficiente para sostener una recomendación, el
  sistema DEBE declararlo explícitamente en lugar de afirmar una conclusión, sin por eso bloquear la
  operación si el usuario decide seguir (Principio IX).

**Confirmación (paso 2, irreversible)**

- **FR-026**: El sistema DEBE aplicar el cambio de presupuesto únicamente al confirmar una propuesta
  existente, vigente y no consumida, identificada por su identificador.
- **FR-027**: Una propuesta confirmada DEBE consumirse: un segundo intento de confirmación sobre la
  misma propuesta DEBE rechazarse sin producir efecto.
- **FR-028**: Si la aplicación del cambio falla, el sistema NO DEBE consumir la propuesta, para
  permitir reintentar sin volver a proponer.
- **FR-029**: El sistema DEBE registrar toda confirmación en el log de auditoría estructurado con,
  como mínimo: momento, nivel y entidad afectada (con la campaña a la que pertenece), campo
  modificado, valor anterior, valor nuevo y quién confirmó (Principio IV).
- **FR-030**: El sistema DEBE detectar que el presupuesto actual cambió entre la propuesta y la
  confirmación, y rechazar la confirmación explicando que la base cambió y hay que proponer de nuevo.
- **FR-031**: Confirmar una propuesta sobre un conjunto de anuncios NO DEBE alterar el presupuesto de
  los demás conjuntos de la misma campaña.

**Errores y mensajes**

- **FR-032**: Todo error orientado al usuario DEBE estar en español y expresado en términos del
  dominio (campaña, conjunto, presupuesto, pesos por día), sin códigos HTTP, stack traces ni jerga
  técnica (Principio VII).
- **FR-033**: El sistema DEBE distinguir y comunicar de forma diferenciada, al menos: entidad
  inexistente, nivel de presupuesto equivocado, falta de permiso para administrar anuncios, monto por
  debajo del mínimo de Meta, límite de seguridad superado, propuesta vencida, propuesta inexistente y
  límite de tasa alcanzado (Principio V).
- **FR-034**: Los montos DEBEN presentarse al usuario en pesos argentinos con formato local, nunca
  en unidades internas de la API.
- **FR-035**: Ningún mensaje, log o respuesta DEBE contener el token de acceso ni credenciales
  (Principio I).

### Key Entities

- **Propuesta de presupuesto**: cambio calculado pero no aplicado. Atributos: identificador, nivel
  (campaña o conjunto), entidad afectada, campaña contenedora, tipo de presupuesto, monto actual,
  monto propuesto, variación, momento de creación, vencimiento. Es de un solo uso.
- **Presupuesto**: monto asignado a una campaña o a un conjunto de anuncios, con un tipo asociado
  (diario o total) y una moneda. Para una misma campaña vive en un nivel o en el otro, nunca en los
  dos a la vez.
- **Conjunto de anuncios**: agrupación dentro de una campaña que define un público y puede tener su
  propio presupuesto. Atributos relevantes: identificador, nombre, estado, campaña a la que
  pertenece, presupuesto vigente.
- **Contexto de rendimiento**: lectura del rendimiento reciente de la entidad (ROAS, gasto, compras,
  período) que acompaña a la propuesta, con su nivel de confianza según el volumen de datos.
- **Registro de auditoría**: constancia estructurada de un cambio efectivamente aplicado.
- **Límites de seguridad**: factor máximo de aumento por operación y techo absoluto de gasto diario,
  ambos configurables, que acotan cuánto puede crecer el gasto.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El usuario puede cambiar el presupuesto de una campaña, de principio a fin y hablando
  en español, en menos de 2 minutos y sin conocer identificadores técnicos ni entrar al
  administrador de anuncios de Meta.
- **SC-002**: El 100 % de los cambios de presupuesto aplicados tiene una propuesta previa
  correspondiente; ninguna vía alternativa permite modificar el presupuesto.
- **SC-003**: El 100 % de los cambios aplicados queda registrado en la auditoría con valor anterior,
  valor nuevo, campaña, momento y responsable, y esos registros permiten reconstruir el historial de
  cambios de un período sin consultar a Meta.
- **SC-004**: Antes de confirmar, el usuario ve siempre el monto actual, el monto resultante y su
  proyección mensual; ninguna confirmación se produce sobre información incompleta.
- **SC-005**: Ningún cambio propuesto puede multiplicar el presupuesto vigente por más del factor
  máximo configurado ni superar el techo de gasto diario; los intentos que lo hagan se rechazan
  indicando cuál límite se superó y cuál es el máximo admitido.
- **SC-008**: El usuario puede ajustar el presupuesto de cualquier campaña de la cuenta, viva la
  plata a nivel campaña o a nivel conjunto de anuncios, sin necesitar saber de antemano en cuál de
  los dos niveles está.
- **SC-009**: Un ajuste expresado en porcentaje siempre se confirma sobre la cifra final en pesos ya
  calculada y redondeada; en el 100 % de los casos el monto aplicado coincide exactamente con el
  monto mostrado antes de confirmar.
- **SC-006**: Las condiciones de error identificadas (campaña inexistente, sin permiso, monto por
  debajo del mínimo, propuesta vencida, base cambiada) producen mensajes distintos y comprensibles
  para una persona no técnica, verificado sobre el conjunto completo de casos.
- **SC-007**: Cuando el rendimiento de la campaña no alcanza el mínimo de 2x, o cuando no hay datos
  suficientes, la propuesta lo advierte explícitamente antes de la confirmación en el 100 % de los
  casos.

## Decisiones tomadas (clarificación 2026-07-28)

- **Alcance de niveles**: la feature cubre **campaña y conjunto de anuncios**. Se decidió no limitarse
  al nivel campaña porque, si las campañas de la cuenta reparten el presupuesto en sus conjuntos, una
  versión sólo-campaña no aplicaría a ninguna campaña real.
- **Forma del cambio**: se aceptan **monto absoluto y ajuste relativo**. El porcentaje se resuelve
  del lado del servidor contra el presupuesto leído en el momento, y la confirmación siempre es sobre
  la cifra absoluta ya calculada.
- **Guardrails**: **doble freno** — factor máximo de aumento por operación (3x por defecto) más techo
  absoluto de gasto diario, ambos configurables. Cubren tanto el error de tipeo puntual como el
  escalamiento acumulado por cambios sucesivos.

## Assumptions

- **Se reutiliza el cimiento de la 009**: el mecanismo de propuestas de un solo uso, su vigencia y
  el log de auditoría ya existen y no se rediseñan; esta feature agrega un tipo de propuesta nuevo
  sobre esa misma base.
- **Vigencia de la propuesta**: se mantiene el mismo tiempo que ya usa el cambio de estado
  (5 minutos), por coherencia, salvo indicación en contrario.
- **Moneda**: la cuenta opera en pesos argentinos; no se contempla conversión de moneda ni cuentas
  multi-moneda.
- **Un solo usuario**: opera una única persona (la dueña del negocio) a través de un cliente MCP
  autenticado; no hay roles ni permisos diferenciados dentro del sistema.
- **Período de referencia del rendimiento**: se usa el mismo período por defecto que ya emplean las
  lecturas de insights existentes.
- **No se cambia el tipo de presupuesto**: esta feature ajusta el **monto** del presupuesto que la
  campaña ya tiene; convertir una campaña de presupuesto diario a total (o viceversa) queda fuera
  de alcance.
- **Sin programación diferida**: los cambios se aplican en el momento de confirmar; no hay
  presupuestos programados a futuro ni reglas automáticas.
- **Sin deshacer automático**: revertir un cambio se hace proponiendo el cambio inverso; no existe
  una operación de "deshacer" dedicada.
- **Precondición operativa**: requiere que el token cargado en el servidor tenga permiso de
  administración de anuncios y que el endpoint esté protegido por autenticación (008 y 009 ya
  cubiertos).

## Dependencies

- **009 — Cimiento de escritura** (propose/confirm + auditoría + puerto de escritura): precondición
  dura. Esta feature no es viable sin él.
- **008 — Autenticación del endpoint**: precondición de seguridad para cualquier escritura expuesta.
- **002 / 005 — Lectura de insights**: necesaria para el contexto de rendimiento de la propuesta
  (User Story 2).
