---

description: "Task list — Feature 010: Ajustar presupuesto de campañas y conjuntos de anuncios"
---

# Tasks: Ajustar presupuesto de campañas y conjuntos de anuncios

**Input**: Design documents from `/specs/010-ajustar-presupuesto-campana/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md)

**Tests**: **SÍ, obligatorios.** La constitución del proyecto exige que las operaciones de escritura
(Principio II) y el manejo de errores de Meta (Principio V) estén cubiertos con tests antes de
marcarse como completos. Cada fase sigue RED → GREEN → REFACTOR.

**Organization**: agrupadas por user story para poder entregar e ir validando de a incrementos.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: puede correr en paralelo (archivos distintos, sin dependencias pendientes)
- **[Story]**: a qué user story pertenece (US1, US2, US3, US4)

## Path Conventions

Proyecto Go único con arquitectura hexagonal. Rutas reales del repo:
`internal/domain/`, `internal/ports/`, `internal/app/`, `internal/adapters/{meta,mcp,memstore}/`,
`internal/config/`, `cmd/server/`. Los tests viven junto al código (`*_test.go`), no en `tests/`.

---

## Phase 1: Setup

**Purpose**: partir de una base verde y aislada.

- [X] T001 Crear y cambiar a la rama `010-ajustar-presupuesto-campana` desde `main`
- [X] T002 Verificar la base en verde y registrar la cobertura de partida: `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test -race ./...`, `go test -cover ./...`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: los tipos de dinero, presupuesto, guardrails y la extensión de `Proposal` que **todas**
las user stories necesitan. Es puro dominio + config: sin red, sin Meta.

**⚠️ CRÍTICO**: ninguna user story puede empezar hasta que esta fase esté completa.

### Tests primero (RED)

- [X] T003 [P] Escribir tests de `Money`, `BudgetType`, `Budget` y `BudgetChange` en `internal/domain/budget_test.go`: parseo desde string en unidades menores, `"0"` y campo ausente como "sin presupuesto", conversión a pesos, aplicación de un cambio relativo con redondeo, rechazo de monto cero/negativo, rechazo de absoluto y relativo simultáneos (FR-006, FR-011 a FR-015)
- [X] T004 [P] Escribir tests de `Guardrails` y `GuardrailError` en `internal/domain/guardrails_test.go`: aumento dentro del factor permitido, aumento que lo excede, resultado que excede el techo diario, bajas siempre permitidas, y que el error transporte el límite violado y el máximo admitido (FR-016 a FR-018)
- [X] T005 [P] Escribir tests de la extensión de `Proposal` en `internal/domain/proposal_test.go`: una propuesta de presupuesto conserva nivel, entidad y `BudgetDelta`; las propuestas de estado existentes siguen construyéndose igual (no romper 009)
- [X] T006 [P] Escribir tests de `Campaign.Budget` en `internal/domain/domain_test.go`: `nil` significa que el presupuesto vive en los conjuntos (FR-007, decisión D2 del plan)
- [X] T007 [P] Escribir tests de las variables `BUDGET_MAX_INCREASE_FACTOR` y `BUDGET_MAX_DAILY_ARS` en `internal/config/config_test.go`: defaults `3.0` y `25000`, override válido, y fail-fast explícito ante valor no numérico o negativo

### Implementación (GREEN)

- [X] T008 Implementar `Money`, `BudgetType`, `Budget`, `BudgetLevel` y `BudgetChange` en `internal/domain/budget.go`, con `Money.Cents int64` y la constante de unidades menores documentada (decisión D1)
- [X] T009 Implementar `Guardrails.Check` y el tipo `GuardrailError` en `internal/domain/guardrails.go` (decisión D4)
- [X] T010 Extender `Proposal` en `internal/domain/proposal.go` con `Level`, `EntityID`, `EntityName`, `Budget *BudgetDelta` y la constante `ProposalBudget`, de forma aditiva (decisión D3)
- [X] T011 Agregar `Budget *Budget` a `Campaign` en `internal/domain/campaign.go`
- [X] T012 [P] Agregar los errores sentinela del dominio (`ErrBudgetLevelMismatch`, `ErrNoBudgetToScale`, `ErrBothAmountAndPercent`, `ErrBudgetUnchanged`, `ErrBudgetDrifted`, `ErrNoAdSets`) en `internal/domain/errors.go`
- [X] T013 Cargar los guardrails desde el entorno en `internal/config/config.go`, siguiendo el patrón de `loadBusinessRules` con `floatEnv`/`intEnv` y fail-fast
- [X] T014 Extender `messageForError` en `internal/adapters/mcp/present.go` para resolver los casos nuevos con `errors.As`/`errors.Is` antes de caer al `switch` por `Kind`, incluyendo el mensaje de guardrail con la cifra real del límite (decisión D7, FR-032, FR-033), y cubrirlo en `internal/adapters/mcp/present_test.go`
- [X] T015 Agregar `formatMoney` en `internal/adapters/mcp/present.go` para presentar montos en pesos con formato local, más la proyección mensual (FR-019, FR-034)

**Checkpoint**: dominio, config y traducción de mensajes listos. `go test -race ./...` en verde.

---

## Phase 3: User Story 1 - Ajustar el presupuesto de una campaña (Priority: P1) 🎯 MVP

**Goal**: cerrar el bucle completo "veo que anda → le pongo plata" a nivel campaña, con propuesta sin
efecto y confirmación irreversible auditada.

**Independent Test**: pedir un cambio de presupuesto sobre una campaña con presupuesto propio;
verificar que tras el propose el monto en Meta sigue igual, y que sólo tras el confirm cambia y queda
en la auditoría.

### Tests para User Story 1 (RED) ⚠️

> Escribir estos tests PRIMERO y verificar que fallan antes de implementar.

- [X] T016 [P] [US1] Tests del mapper en `internal/adapters/meta/budget_test.go`: `parseCampaign`/`parseCampaigns` con `daily_budget` presente, con `lifetime_budget`, con `"0"` y con el campo ausente; verificar que los tests existentes de campañas siguen pasando con los campos nuevos (R7 del research)
- [X] T017 [P] [US1] Tests de `UpdateCampaignBudget` con `httptest` en `internal/adapters/meta/write_test.go`: que el POST vaya al nodo correcto, que mande `daily_budget` en unidades menores, y que un error de Meta se traduzca a error semántico sin exponer el token
- [X] T018 [P] [US1] Tests de `ProposeBudget` a nivel campaña en `internal/app/propose_budget_test.go`: monto absoluto, ajuste relativo, monto igual al actual rechazado, cero/negativo rechazado, absoluto+relativo simultáneos rechazado, campaña archivada rechazada, guardrail excedido rechazado, y que **no** se llame al writer en ningún caso (FR-001 a FR-006, FR-011 a FR-021)
- [X] T019 [P] [US1] Tests de `ConfirmProposal` para `ProposalBudget` en `internal/app/write_test.go`: aplica el cambio, consume la propuesta, no la consume si el writer falla, rechaza propuesta vencida, rechaza segunda confirmación, y registra la auditoría con nivel, entidad, antes y después (FR-026 a FR-030)

### Implementación para User Story 1 (GREEN)

- [X] T020 [US1] Extender `campaignFields` con `daily_budget,lifetime_budget` e implementar `parseBudget` en `internal/adapters/meta/mapper.go`
- [X] T021 [US1] Agregar `UpdateCampaignBudget` a `ports.MetaWriter` en `internal/ports/meta.go` e implementarlo en `internal/adapters/meta/client.go` reutilizando `c.post()`
- [X] T022 [US1] Implementar el caso de uso `ProposeBudget` en `internal/app/propose_budget.go` (nivel campaña): lectura del presupuesto actual, resolución absoluto/relativo con redondeo, chequeo de guardrails, construcción y guardado de la propuesta
- [X] T023 [US1] Extender `ConfirmProposal.apply()` en `internal/app/confirm_proposal.go` con el caso `ProposalBudget`, incluyendo la relectura para detectar deriva (decisión D6) y el registro de auditoría con `nivel` y `entidad_id`
- [X] T024 [US1] Definir la tool `propose_budget` y su handler en `internal/adapters/mcp/tools.go`, con `ReadOnlyHint(true)` y `DestructiveHint(false)`, parámetros `campaign_id`, `amount_ars`, `percent_change`
- [X] T025 [US1] Ramificar `formatProposal` por `Kind` y agregar el formato de confirmación de presupuesto en `internal/adapters/mcp/present.go`, mostrando monto actual, monto resultante, variación, proyección mensual y si genera gasto inmediato (FR-003, FR-004, FR-019)
- [X] T026 [US1] Agregar `ProposeBudget` a `Deps` y registrar la tool en `internal/adapters/mcp/server.go`, y cablear el caso de uso en `cmd/server/main.go`
- [X] T027 [US1] Extender `internal/adapters/mcp/tools_test.go`: la tool nueva aparece en el registro, `propose_budget` está anotada como no destructiva y `confirm_action` sigue siendo la única destructiva

**Checkpoint**: MVP entregable. Se puede cambiar el presupuesto de una campaña de punta a punta.

---

## Phase 4: User Story 2 - Ajustar el presupuesto de un conjunto de anuncios (Priority: P2)

**Goal**: que la feature aplique aunque la plata viva en los conjuntos, y que el sistema sepa
redirigir solo en vez de fallar.

**Independent Test**: sobre una campaña que reparte presupuesto en conjuntos, pedir el cambio a nivel
campaña y verificar que el sistema lista los conjuntos con sus montos; luego ajustar uno y verificar
que los otros quedan intactos.

### Tests para User Story 2 (RED) ⚠️

- [X] T028 [P] [US2] Tests de `parseAdSets`, `GetAdSets` y `GetAdSet` con `httptest` en `internal/adapters/meta/adset_test.go`, incluyendo campaña sin conjuntos
- [X] T029 [P] [US2] Tests de `UpdateAdSetBudget` con `httptest` en `internal/adapters/meta/write_test.go`
- [X] T030 [P] [US2] Tests de `GetBudgets` en `internal/app/get_budgets_test.go`: campaña con presupuesto propio, campaña que reparte en conjuntos, campaña sin conjuntos (FR-009, FR-010)
- [X] T031 [P] [US2] Tests de resolución de nivel en `internal/app/propose_budget_test.go`: pedir campaña cuando la plata está en los conjuntos devuelve la lista de conjuntos; pedir conjunto cuando la campaña controla el presupuesto se rechaza con la explicación inversa (FR-007, FR-008)
- [X] T032 [P] [US2] Test en `internal/app/write_test.go`: confirmar una propuesta sobre un conjunto no altera el presupuesto de los demás conjuntos de la campaña (FR-031)

### Implementación para User Story 2 (GREEN)

- [X] T033 [P] [US2] Crear la entidad `AdSet` en `internal/domain/adset.go`
- [X] T034 [US2] Agregar `GetAdSets` y `GetAdSet` a `ports.MetaReader` y `UpdateAdSetBudget` a `ports.MetaWriter` en `internal/ports/meta.go`
- [X] T035 [US2] Implementar los tres métodos en `internal/adapters/meta/client.go` con `adSetFields` y `parseAdSets` en `internal/adapters/meta/mapper.go`
- [X] T036 [US2] Actualizar los dobles de prueba (`fakeReader` en `internal/app/` y en `internal/adapters/mcp/`, `noopWriter`) para satisfacer los puertos ampliados
- [X] T037 [US2] Implementar el caso de uso `GetBudgets` en `internal/app/get_budgets.go`
- [X] T038 [US2] Extender `ProposeBudget` en `internal/app/propose_budget.go` con la resolución de nivel y la redirección que lista los conjuntos
- [X] T039 [US2] Extender `ConfirmProposal.apply()` con el caso de conjunto en `internal/app/confirm_proposal.go`
- [X] T040 [US2] Definir la tool `get_budgets` (sólo lectura) y su handler en `internal/adapters/mcp/tools.go`, y agregar `adset_id` como parámetro alternativo de `propose_budget`
- [X] T041 [US2] Implementar `formatBudgets` en `internal/adapters/mcp/present.go`: dónde vive la plata, lista de conjuntos con nombre, estado y monto
- [X] T042 [US2] Registrar la tool y cablear `GetBudgets` en `internal/adapters/mcp/server.go` y `cmd/server/main.go`; extender `internal/adapters/mcp/tools_test.go`

**Checkpoint**: US1 y US2 funcionan de forma independiente. La feature aplica a cualquier campaña.

---

## Phase 5: User Story 3 - Decidir con el rendimiento a la vista (Priority: P2)

**Goal**: que la propuesta traiga el ROAS evaluado y sea honesta cuando no hay datos suficientes.

**Independent Test**: proponer sobre una campaña con buen ROAS y sobre otra con volumen ínfimo, y
verificar que la primera muestra el ROAS contra el umbral de 2x y la segunda advierte datos
insuficientes sin bloquear la confirmación.

### Tests para User Story 3 (RED) ⚠️

- [X] T043 [P] [US3] Tests en `internal/app/propose_budget_test.go`: la propuesta incluye el rendimiento evaluado, marca bajo rendimiento por debajo de 2x, advierte datos insuficientes y **no** bloquea la confirmación en ese caso (FR-023 a FR-025)
- [X] T044 [P] [US3] Test en `internal/adapters/mcp/present_test.go`: el bloque de rendimiento aparece en el texto de la propuesta con el ícono de estado correspondiente

### Implementación para User Story 3 (GREEN)

- [X] T045 [US3] Extender `ProposeBudget` para consultar insights de la entidad y evaluarlos con el `Evaluate` existente de `internal/app/evaluate.go`, reutilizando los umbrales de la config (Principio VIII)
- [X] T046 [US3] Incluir el bloque de rendimiento en `formatProposal` en `internal/adapters/mcp/present.go`, reutilizando `metricsReport` en vez de duplicar el formateo
- [X] T047 [US3] Acotar el paso propose a 3 llamadas a Meta como máximo y verificarlo en el test, para no degradar el rate limiting (Principio VI)

**Checkpoint**: las tres primeras stories funcionan y la decisión es informada.

---

## Phase 6: User Story 4 - Reasignar presupuesto entre campañas (Priority: P3)

**Goal**: que bajar en una y subir en otra funcione como dos operaciones independientes, sin
arrastre entre confirmaciones.

**Independent Test**: crear dos propuestas sobre campañas distintas, confirmar sólo una, y verificar
que la otra no produjo ningún efecto.

### Tests para User Story 4 (RED) ⚠️

- [X] T048 [P] [US4] Test en `internal/app/write_test.go`: dos propuestas coexisten en el store, confirmar una no consume ni aplica la otra, y cada una se audita por separado
- [X] T049 [P] [US4] Test de concurrencia en `internal/adapters/memstore/store_test.go` con `-race`: varias propuestas guardadas y consumidas en paralelo

### Implementación para User Story 4 (GREEN)

- [X] T050 [US4] Ajustar lo que haga falta para que los tests anteriores pasen, y explicitar el patrón de reasignación en la descripción de la tool `propose_budget` para que el cliente MCP sepa encadenar las dos operaciones

**Checkpoint**: las cuatro user stories completas.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T051 Verificación completa: `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test -race ./...`, `go test -cover ./...` con cobertura ≥ 80 %
- [X] T052 Revisión de seguridad: confirmar que ningún log, mensaje ni respuesta incluye el token, y que `ConfirmProposal` sigue siendo el único camino que invoca al writer (Principios I, II, III, IV)
- [X] T053 Repasar los 35 FRs de `spec.md` y marcar dónde queda cubierto cada uno, o justificar explícitamente el que no lo esté
- [X] T054 [P] Actualizar `docs/ROADMAP.md`: marcar la 010 como hecha y anotar el pendiente de redeploy
- [X] T055 [P] Documentar `BUDGET_MAX_INCREASE_FACTOR` y `BUDGET_MAX_DAILY_ARS` en `README.md` junto al resto de las variables de entorno
- [X] T056 Cerrar o dejar registrada la incógnita R5 de `research.md` (código de error de presupuesto mínimo), sin inventar un código
- [X] T057 Commit siguiendo el formato del proyecto y verificación de que no hay secretos en el diff

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Fase 1)**: sin dependencias
- **Foundational (Fase 2)**: depende de Setup — **BLOQUEA todas las user stories**
- **US1 (Fase 3)**: depende de Fase 2. Es el MVP
- **US2 (Fase 4)**: depende de Fase 2. Reutiliza el caso de uso de US1, así que en la práctica conviene después de US1
- **US3 (Fase 5)**: depende de Fase 2 y de que exista `ProposeBudget` (US1)
- **US4 (Fase 6)**: depende de US1; es mayormente verificación de que la composición ya funciona
- **Polish (Fase 7)**: depende de todas las stories que se decidan entregar

### Dentro de cada story

- Tests primero, verificados en rojo, antes de implementar
- Dominio → puertos → adaptador de Meta → caso de uso → presentación MCP → cableado

### Parallel Opportunities

- **Fase 2**: T003 a T007 en paralelo (archivos distintos); luego T008 a T011 en secuencia por tocar tipos relacionados; T012 en paralelo
- **Fase 3**: T016 a T019 en paralelo (cuatro archivos de test distintos)
- **Fase 4**: T028 a T032 en paralelo; T033 en paralelo con los tests
- **Fase 5**: T043 y T044 en paralelo
- **Fase 7**: T054 y T055 en paralelo

## Parallel Example: User Story 1

```bash
# Los cuatro archivos de test de US1 son independientes entre sí:
T016  internal/adapters/meta/budget_test.go     # mapper de presupuesto
T017  internal/adapters/meta/write_test.go      # POST de presupuesto
T018  internal/app/propose_budget_test.go       # caso de uso propose
T019  internal/app/write_test.go                # caso de uso confirm
```

## Implementation Strategy

### MVP primero (sólo User Story 1)

1. Fase 1: Setup
2. Fase 2: Foundational (crítica, bloquea todo)
3. Fase 3: User Story 1
4. **PARAR Y VALIDAR**: probar el cambio de presupuesto de una campaña de punta a punta
5. Recién ahí decidir si sigue US2

### Entrega incremental

1. Setup + Foundational → base lista
2. + US1 → validar → **MVP**
3. + US2 → validar → la feature aplica a cualquier campaña
4. + US3 → validar → la decisión es informada
5. + US4 → validar → reasignación entre campañas

### Nota sobre el orden real

US2 es P2 pero puede volverse urgente: si al correr `get_budgets` contra la cuenta real resulta que
todas las campañas reparten el presupuesto en sus conjuntos, el MVP de US1 no le sirve a nadie y hay
que seguir de largo hasta US2. Vale la pena verificar eso apenas exista T037.

## Notes

- `[P]` = archivos distintos, sin dependencias pendientes
- Verificar que cada test falla antes de implementarlo
- Commit por tarea o por grupo lógico coherente
- Nada de escritura fuera de `ConfirmProposal`: si una tarea parece necesitarlo, está mal planteada
