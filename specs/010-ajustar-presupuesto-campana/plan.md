# Implementation Plan: Ajustar presupuesto de campañas y conjuntos de anuncios

**Branch**: `010-ajustar-presupuesto-campana` | **Date**: 2026-07-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/010-ajustar-presupuesto-campana/spec.md`

## Summary

Agregar la capacidad de **leer y modificar el presupuesto** de una campaña o de un conjunto de
anuncios, reutilizando el cimiento propose/confirm de la 009 sin abrir un segundo camino de
escritura. La feature introduce tres piezas nuevas de dominio (dinero, presupuesto, conjunto de
anuncios), un tipo de propuesta nuevo (`ProposalBudget`) que convive con el de estado, dos
operaciones de lectura y dos de escritura en el puerto de Meta, y un par de guardrails configurables
que acotan cuánto puede crecer el gasto en una operación.

El enfoque técnico es aditivo: nada de lo que ya funciona se rediseña. `ConfirmProposal` gana un
caso en su `switch`, `domain.Proposal` gana campos opcionales, `MetaReader`/`MetaWriter` ganan
métodos, y la tool `confirm_action` sirve al nuevo flujo sin cambios porque ya es genérica.

## Technical Context

**Language/Version**: Go 1.23 (ver `go.mod`)

**Primary Dependencies**: `github.com/mark3labs/mcp-go` (SDK MCP), biblioteca estándar para HTTP y
logging (`net/http`, `log/slog`). Sin dependencias nuevas.

**Storage**: en memoria (`internal/adapters/memstore`) para propuestas pendientes. Sin base de datos.

**Testing**: `go test` con tests table-driven, `httptest.Server` para el adaptador de Meta, dobles
de prueba (`fakeReader`, `noopWriter`) en `app` y `mcp`. Race detector obligatorio.

**Target Platform**: servidor Linux en Railway, transporte MCP sobre HTTP con bearer token.

**Project Type**: servicio único (servidor MCP), arquitectura hexagonal.

**Performance Goals**: no es una feature de volumen. El costo dominante es la latencia de la Graph
API; el paso propose hace como máximo 3 llamadas (entidad + conjuntos + insights).

**Constraints**: rate limiting obligatorio contra Meta (ya implementado en `meta.Limiter`, se hereda
por usar el mismo `request()`). Escritura irreversible sobre gasto real.

**Scale/Scope**: una cuenta publicitaria, un usuario, decenas de campañas. Presupuesto mensual
menor a USD 500.

## Constitution Check

*GATE: Debe pasar antes de Fase 0 y re-verificarse tras el diseño.*

| Principio | Cómo lo cumple este plan | Estado |
|---|---|---|
| **I — Confidencialidad del token** | Los guardrails y el presupuesto no tocan el token. El adaptador `meta` sigue siendo el único que lo conoce; se reutiliza `do()`, que ya evita loguear URLs. Nada nuevo se agrega a los logs con credenciales. | ✅ Pasa |
| **II — Propose/Confirm (NO NEGOCIABLE)** | `ProposeBudget` es sólo lectura y guarda en `ProposalStore`. `UpdateCampaignBudget`/`UpdateAdSetBudget` se invocan **exclusivamente** desde `ConfirmProposal.apply()`. Ninguna tool nueva escribe. | ✅ Pasa |
| **III — Frontera única con Meta** | Los métodos nuevos se agregan a `ports.MetaReader`/`ports.MetaWriter` e implementan en `meta.Client`. Ningún caso de uso conoce HTTP. | ✅ Pasa |
| **IV — Auditoría de toda escritura** | `ConfirmProposal` ya audita con `slog`; se extiende el registro con `nivel` y `entidad_id` para distinguir campaña de conjunto (FR-029). | ✅ Pasa |
| **V — Errores semánticos** | Errores tipados nuevos en `domain` (guardrail, nivel equivocado, base cambiada). `apply()` no consume la propuesta si falla (FR-028, ya implementado). Sin catch vacíos. | ✅ Pasa |
| **VI — Rate limiting** | Se hereda: todo pasa por `Client.request()`, que ya aplica limiter + backoff. El paso propose agrega llamadas, por eso se acota a 3 como máximo. | ✅ Pasa |
| **VII — Mensajes legibles** | `messageForError` se extiende con `errors.As` para los casos nuevos; el texto en español vive en la capa de presentación, no en el dominio. Montos formateados en pesos. | ✅ Pasa |
| **VIII — Umbrales del negocio** | La propuesta incluye contexto de rendimiento evaluado con `app.Evaluate` (ROAS vs 2x), reutilizando el evaluador existente. | ✅ Pasa |
| **IX — Honestidad ante datos insuficientes** | Se reutiliza `Evaluation.Insufficient`; la propuesta lo muestra como advertencia y **no** bloquea la confirmación (FR-025). | ✅ Pasa |

**Gate de desarrollo**: escritura y errores de Meta cubiertos con tests antes de marcar completo;
revisión de seguridad por tocar la frontera con Meta y el log de auditoría; sin secretos en commits.

## Patterns to Mirror

| Categoría | Fuente | Patrón a copiar |
|---|---|---|
| Nombres de archivo/caso de uso | `internal/app/propose_campaign_status.go` | Un archivo por caso de uso, struct con deps privadas, `New…` + `New…WithClock`, método `Execute` |
| Constructor con opciones | `internal/adapters/meta/client.go:50-68` | `type Option func(*Client)` + `With…` para inyectar en tests |
| Errores | `internal/domain/errors.go:42` | `domain.NewError(Kind, op, cause)` con `const op = "paquete.Func"` al inicio de cada función |
| Traducción de errores de Meta | `internal/adapters/meta/errors.go` | `translateError(status, apiErr, op)` mapea códigos de Meta a `domain.Kind` |
| Mensajes al usuario | `internal/adapters/mcp/present.go:14-27` | `messageForError` con `switch domain.KindOf(err)`; el español vive sólo acá |
| Formateo de salida | `internal/adapters/mcp/present.go:88-107` | `format…(…) string` con `strings.Builder`, viñetas `•`, íconos por estado |
| Esquema de tool | `internal/adapters/mcp/tools.go:233-268` | `mcp.NewTool` + anotaciones `ReadOnlyHint`/`DestructiveHint` + `mcp.Required()` |
| Handler de tool | `internal/adapters/mcp/tools.go:69-99` | Leer params → `uc.Execute` → en error `slog.Error(...)` + `mcp.NewToolResultError(messageForError(err))` |
| Config desde entorno | `internal/config/config.go:91-127` | `loadBusinessRules` con `floatEnv`/`intEnv`, default explícito, fail-fast si el valor es inválido |
| Parseo de respuesta | `internal/adapters/meta/mapper.go` | `parse…(body []byte)` devuelve entidad de dominio; error de parseo → `KindUpstream` |
| Tests de adaptador | `internal/adapters/meta/write_test.go` | `httptest.Server` + `WithBaseURL` + `WithLimiter(NewLimiter(0))` |
| Tests de caso de uso | `internal/app/write_test.go` | `fakeReader`/`noopWriter` + reloj inyectado para probar vencimiento |

## Project Structure

### Documentation (this feature)

```text
specs/010-ajustar-presupuesto-campana/
├── spec.md              # Especificación (hecho)
├── plan.md              # Este archivo
├── research.md          # Fase 0: incógnitas de la Graph API sobre presupuesto
├── tasks.md             # Fase 2 (lo genera /tasks, no este comando)
└── checklists/
    └── requirements.md  # Checklist de calidad de la spec (hecho)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── budget.go          # NUEVO — Money, BudgetType, Budget, BudgetLevel, BudgetChange, Guardrails
│   ├── budget_test.go     # NUEVO
│   ├── adset.go           # NUEVO — AdSet + AdSetQuery
│   ├── campaign.go        # MODIFICADO — Campaign gana Budget *Budget (nil = vive en los ad sets)
│   ├── proposal.go        # MODIFICADO — ProposalBudget, Level, EntityID/Name, Budget *BudgetDelta
│   └── errors.go          # MODIFICADO — GuardrailError tipado + sentinelas del dominio
├── ports/
│   └── meta.go            # MODIFICADO — MetaReader += GetAdSets/GetAdSet; MetaWriter += Update*Budget
├── adapters/
│   ├── meta/
│   │   ├── client.go      # MODIFICADO — campos de presupuesto, GetAdSets, GetAdSet, Update*Budget
│   │   ├── mapper.go      # MODIFICADO — parseAdSets, parseBudget (unidades menores → Money)
│   │   ├── errors.go      # MODIFICADO — código de "presupuesto por debajo del mínimo"
│   │   └── budget_test.go # NUEVO
│   └── mcp/
│       ├── tools.go       # MODIFICADO — get_budgets, propose_budget + handlers
│       ├── present.go     # MODIFICADO — formatBudgets, formatProposal por Kind, mensajes nuevos
│       ├── server.go      # MODIFICADO — Deps += GetBudgets, ProposeBudget; registro de tools
│       └── tools_test.go  # MODIFICADO
├── app/
│   ├── get_budgets.go     # NUEVO — lectura: dónde vive la plata y cuánta hay
│   ├── propose_budget.go  # NUEVO — paso 1, sin efecto
│   ├── confirm_proposal.go# MODIFICADO — apply() gana el caso ProposalBudget + chequeo de deriva
│   └── *_test.go          # NUEVO/MODIFICADO
├── config/
│   └── config.go          # MODIFICADO — Guardrails desde entorno
└── cmd/server/main.go     # MODIFICADO — cablear los casos de uso nuevos
```

**Structure Decision**: se mantiene la hexagonal existente sin capas nuevas. El dominio gana tipos,
los puertos ganan métodos, y cada adaptador implementa lo suyo. No se agregan paquetes salvo los
archivos listados.

## Decisiones de diseño

### D1 — El dinero se modela en unidades menores, nunca en float

`domain.Money{Cents int64}`. La Graph API devuelve `daily_budget`/`lifetime_budget` como **string en
unidades menores** (centavos para ARS). Usar `float64` para plata invita a errores de redondeo sobre
gasto real. La conversión a pesos ocurre sólo en la capa de presentación.

### D2 — `Campaign.Budget == nil` es la señal de "la plata vive en los conjuntos"

Es la forma natural de responder FR-007 sin un flag redundante: si Meta no devuelve presupuesto a
nivel campaña, el presupuesto está en los ad sets. `ProposeBudget` usa eso para decidir si acepta la
operación o si redirige listando los conjuntos.

### D3 — `Proposal` se extiende de forma aditiva

Los campos nuevos (`Level`, `EntityID`, `EntityName`, `Budget *BudgetDelta`) son opcionales. Las
propuestas de estado de la 009 siguen construyéndose igual y sus tests no se tocan. `apply()` sigue
ramificando por `Kind`.

### D4 — Los guardrails viven en el dominio, no en el adaptador

`domain.Guardrails.Check(actual, propuesto Money, t BudgetType) error` es una regla de negocio pura y
testeable sin red ni mocks. Devuelve un `*GuardrailError` tipado con el límite violado y el máximo
admitido, para que la presentación arme un mensaje con cifras reales.

### D5 — El porcentaje se resuelve en el servidor y se confirma en pesos

`ProposeBudget` acepta `amount_ars` **o** `percent_change`, nunca ambos (FR-015). El cálculo se hace
sobre el presupuesto leído en ese mismo momento y se redondea antes de guardar la propuesta, de modo
que el monto aprobado y el aplicado sean idénticos por construcción (SC-009).

### D6 — La deriva se detecta releyendo en el confirm

`ConfirmProposal.apply()` para `ProposalBudget` relee el presupuesto actual y lo compara con
`BudgetDelta.BeforeCents`. Si no coincide, falla con `ErrBudgetDrifted` sin escribir y sin consumir
la propuesta (FR-030). Es la única forma honesta de cumplirlo sin bloqueo optimista del lado de Meta.

### D7 — Mensajes finos vía `errors.As`, no vía más `Kind`s

`messageForError` primero intenta `errors.As` contra los tipos/sentinelas nuevos y sólo cae al
`switch` por `Kind` si no matchea. Evita inflar el enum `Kind` (que es semántica de transporte) con
casos de negocio, y mantiene todo el español en `present.go`.

## Fases de implementación

Cada fase es TDD: test primero (RED) → implementación mínima (GREEN) → refactor. Cada fase deja el
repo compilando y con los tests en verde.

### Fase 0 — Investigación de la Graph API *(bloqueante)*

Resolver las incógnitas de `research.md` antes de escribir el adaptador: formato exacto de los campos
de presupuesto, cómo se distingue CBO de presupuesto por ad set, el código de error de "monto por
debajo del mínimo", y si el POST de presupuesto acepta el mismo patrón que `UpdateCampaignStatus`.
**Salida**: `research.md` con decisiones y fallbacks documentados.

### Fase 1 — Dominio

`Money`, `BudgetType`, `Budget`, `BudgetLevel`, `BudgetChange`, `Guardrails` + `GuardrailError`,
`AdSet`, extensión de `Campaign` y `Proposal`, sentinelas de error. Tests puros, sin red.

### Fase 2 — Puertos y adaptador de Meta

`MetaReader.GetAdSets`/`GetAdSet`, `MetaWriter.UpdateCampaignBudget`/`UpdateAdSetBudget`, campos de
presupuesto en las consultas existentes, `parseAdSets`/`parseBudget`, traducción del error de mínimo.
Tests con `httptest`, incluyendo la aserción de que el POST manda el campo correcto.

### Fase 3 — Casos de uso

`GetBudgets` (lectura), `ProposeBudget` (resolución de nivel, absoluto/relativo, guardrails, contexto
de rendimiento), extensión de `ConfirmProposal` con el caso de presupuesto y la detección de deriva.

### Fase 4 — Configuración

`BUDGET_MAX_INCREASE_FACTOR` (default `3.0`) y `BUDGET_MAX_DAILY_ARS` (default `25000`, coherente con
< USD 500/mes). Fail-fast ante valores inválidos, siguiendo `loadBusinessRules`.

### Fase 5 — Presentación MCP

Tools `get_budgets` y `propose_budget` con sus anotaciones, handlers, `formatBudgets`,
`formatProposal` ramificado por `Kind`, `formatConfirmation` para presupuesto, y los mensajes de
error nuevos. `confirm_action` **no se toca**.

### Fase 6 — Cableado y verificación

`main.go`, `Deps`, y la corrida completa: `go build ./...`, `go vet ./...`, `go test -race ./...`,
cobertura ≥ 80 %.

### Fase 7 — Documentación

Actualizar `docs/ROADMAP.md` (010 → hecha) y documentar las variables de entorno nuevas.

## Validation

```bash
gofmt -l .                       # sin salida = formateado
go build ./...
go vet ./...
go test -race ./...
go test -cover ./...             # objetivo ≥ 80 %
```

## Risks

| Riesgo | Prob. | Impacto | Mitigación |
|---|---|---|---|
| El formato/unidad de los campos de presupuesto de Meta no es el asumido | Media | Alto — montos mal escritos = plata real mal gastada | Fase 0 bloqueante; test de mapper con payload real capturado; el propose siempre muestra el monto en pesos antes de confirmar |
| No identificar el código de error de "monto por debajo del mínimo" | Media | Medio — mensaje genérico en vez de específico | Fallback documentado: si no se identifica, el mensaje explica que Meta rechazó el monto por ser menor al mínimo permitido y sugiere subirlo. No se inventa un código |
| La cuenta usa presupuesto a nivel ad set y el flujo de campaña queda sin uso real | Alta | Bajo — ya está contemplado en el alcance | La User Story 2 cubre ad sets; el propose redirige solo. Verificar contra la cuenta real apenas haya `get_budgets` |
| Falta el permiso `ads_management` en el token de producción | Alta | Alto — la escritura falla en vivo | Ya es un pendiente conocido de la 009; el mensaje de error lo dice explícitamente. Verificar antes de la prueba en vivo |
| Deriva entre propose y confirm en uso real | Baja | Medio | D6: relectura y rechazo explícito |
| Crecimiento de `Proposal` hacia un tipo con demasiadas responsabilidades | Media | Bajo | Campos nuevos opcionales y agrupados en `BudgetDelta`; si aparece un tercer tipo de propuesta, extraer una interfaz |

## Complexity Tracking

| Violación potencial | Por qué se necesita | Alternativa más simple, y por qué se descartó |
|---|---|---|
| Dos niveles de escritura (campaña + conjunto) en una sola feature | Decisión explícita del usuario en la clarificación: si las campañas reparten presupuesto en sus conjuntos, una versión sólo-campaña no aplicaría a ninguna campaña real de la cuenta | Limitarse a campañas (CBO). Descartado por el usuario: entrega rápido pero con riesgo alto de no servir para nada |
| Dos formas de expresar el cambio (absoluto + relativo) | El usuario final habla en porcentajes ("subile 30 %"); resolverlo en el servidor evita que el cliente MCP haga aritmética sobre plata con datos posiblemente viejos | Sólo absoluto (KISS). Descartado: traslada el cálculo a un lugar donde no se puede garantizar que el presupuesto base esté fresco |
| Tipo `Money` propio en vez de `float64` | Aritmética exacta sobre dinero real e irreversible | `float64` como en las métricas de lectura. Descartado: en lectura un centavo de error es cosmético, en escritura es plata mal gastada |
| `GuardrailError` tipado en vez de un `Kind` nuevo | El mensaje debe incluir el límite violado y el máximo admitido, cosa que un `Kind` no transporta | Agregar `KindGuardrail`. Descartado: `Kind` clasifica semántica de transporte, no reglas de negocio con parámetros |

## Acceptance

- [ ] Fase 0 cerrada: `research.md` sin incógnitas abiertas que bloqueen el adaptador
- [ ] Las 7 fases completas, cada una con sus tests en verde
- [ ] `gofmt`, `go build`, `go vet`, `go test -race` sin hallazgos
- [ ] Cobertura ≥ 80 %
- [ ] Constitution Check re-verificado después del diseño
- [ ] Los 35 FRs de la spec tienen cobertura de test o justificación explícita
- [ ] Ningún camino de escritura nuevo fuera de `ConfirmProposal`
