# Implementation Plan: Insights de Decisión (ROAS/CPA + Desglose de Audiencias)

**Branch / Feature dir**: `specs/002-insights-roas-audience` | **Date**: 2026-06-24 | **Spec**: [spec.md](./spec.md)

## Summary

Extender la lectura de rendimiento sobre la arquitectura hexagonal existente (feature 001):
enriquecer `get_campaigns_insights` con métricas de conversión (ROAS, compras, facturación,
CPA, CTR de enlace, frecuencia) evaluadas contra los umbrales de negocio, y agregar la tool
`get_audience_breakdown` que segmenta esas métricas por dimensión. Todo estrictamente de
solo lectura. La evaluación contra umbrales y la política de "datos insuficientes" viven en la
capa `app`; la presentación con íconos vive en el adaptador `mcp`.

## Technical Context

**Language/Version**: Go 1.25 (toolchain auto-gestionado por mcp-go v0.55.0).

**Primary Dependencies**: `github.com/mark3labs/mcp-go` (MCP), `github.com/joho/godotenv` (env local). Sin dependencias nuevas previstas.

**Storage**: N/A (sin persistencia; estado efímero por request).

**Testing**: `go test` table-driven + `httptest` para el adaptador Meta; `-race` y cobertura.

**Target Platform**: contenedor Linux (Docker) en Railway; transporte Streamable HTTP sobre `$PORT`.

**Project Type**: servicio único (servidor MCP).

**Performance/Constraints**: respetar rate limits de Meta (Principio VI, ya implementado); respuestas legibles en español (Principio VII).

**Scale/Scope**: 1 cuenta publicitaria; campañas en el orden de cientos; 2 tools de lectura.

## Constitution Check

*GATE: revisado contra `.specify/memory/constitution.md` v1.1.0.*

| Principio | Cómo lo cumple este plan |
|-----------|--------------------------|
| I — Token confinado | Sin cambios; el token sigue solo en `meta` adapter vía config. |
| III — Frontera única | Las nuevas lecturas y `breakdowns` pasan solo por `ports.MetaReader`. |
| V — Errores semánticos | Combos de segmentación inválidos → `KindInvalidInput`; vacío ≠ error; nunca éxito falso. |
| VI — Rate limiting | Reusa el limiter + backoff existente del cliente Meta. |
| VII — Mensajes en español | Estados ✅/⚠️/❌ y mensajes de error en la capa de presentación. |
| VIII — Umbrales | ROAS 2x, CTR enlace 0,8–1,5%, frecuencia 3,5, CPA vs ticket (≈80.000 ARS), ajustables por config. |
| IX — Datos insuficientes (NO NEGOCIABLE) | Métricas de decisión nullables (`*float64`/`*int64`): nil = "no calculable", nunca 0. Política de suficiencia en `app`. |
| Solo lectura | Cero operaciones de escritura; no se incorpora puerto de escritura ni propose/confirm. |

**Sin violaciones que justificar** → Complexity Tracking vacío.

## Project Structure

### Documentation (this feature)

```text
specs/002-insights-roas-audience/
├── spec.md                  # Especificación
├── plan.md                  # Este archivo
└── checklists/
    └── requirements.md      # Checklist de calidad del spec
```

### Source Code (cambios sobre la estructura existente)

```text
internal/
├── config/config.go         # + umbrales (ROAS/CTR/frecuencia/ticket) y suficiencia
├── domain/
│   ├── insight.go           # Metrics extendido (campos de conversión nullables)
│   ├── metric_status.go     # NUEVO: MetricStatus + Thresholds + SufficiencyPolicy
│   └── audience.go          # NUEVO: BreakdownDimension, AudienceSegment, AudienceBreakdown, AudienceQuery
├── ports/meta.go            # + GetAudienceBreakdown
├── app/
│   ├── get_insights.go      # devuelve evaluación (insight + estados + suficiencia)
│   ├── evaluate.go          # NUEVO: aplicar umbrales + suficiencia a Metrics
│   └── get_audience_breakdown.go  # NUEVO caso de uso
├── adapters/meta/
│   ├── client.go            # + fields de conversión; método GetAudienceBreakdown (breakdowns=)
│   ├── mapper.go            # parsear actions/action_values/cost_per_action_type/purchase_roas (arrays)
│   └── errors.go            # traducir combos inválidos → KindInvalidInput
└── adapters/mcp/
    ├── tools.go             # nueva tool get_audience_breakdown (annotations read-only)
    └── present.go           # render de métricas de conversión con íconos + "no calculable"
```

**Structure Decision**: se mantiene la hexagonal de la feature 001; solo se extienden capas
existentes y se agregan archivos nuevos cohesivos por responsabilidad. Sin reorganización.

## Decisiones confirmadas

1. **Definición de "compra"**: usar el `action_type` consolidado `purchase` si está presente;
   si no, caer a `offsite_conversion.fb_pixel_purchase` / `onsite_web_purchase`. Evita doble conteo.
2. **Nivel público granular (ad set)**: fuera de esta iteración. Solo campaña y cuenta.
3. **Umbrales de "datos insuficientes" (defaults, ajustables por config)**:
   - ROAS/CPA se calculan solo con **≥ 1 compra** y **≥ 7 días** de período.
   - Métricas de tasa (CTR enlace, frecuencia) → "sin datos" si **< 1000 impresiones** o **< 50 clics de enlace**.
4. **Ticket promedio**: default **80.000 ARS** (Tienda Nube), override por config; cuando hay
   datos del período puede derivarse facturación ÷ compras.

## Phases

| Fase | Contenido | TDD |
|------|-----------|-----|
| 0 | Research: sondeo en vivo del JSON real de `actions`/`action_values`/`cost_per_action_type`/`purchase_roas` + forma de respuesta con `breakdowns`. | — |
| 1 | `config`: umbrales + suficiencia con defaults; parseo float/int. | sí |
| 2 | `domain`: Metrics nullables, MetricStatus, Thresholds, SufficiencyPolicy, tipos de audiencia. | sí |
| 3 | `adapters/meta`: fields nuevos, parseo de arrays, GetAudienceBreakdown, combos inválidos → KindInvalidInput. | sí (httptest) |
| 4 | `app`: evaluación de umbrales + suficiencia en GetInsights; GetAudienceBreakdown. | sí |
| 5 | `adapters/mcp`: presentación con íconos + "no calculable"; tool nueva con annotations read-only. | sí |
| 6 | `cmd/server`: inyectar umbrales desde config. | — |
| 7 | Verificación: `go vet`, `go test -race`, cobertura ≥ 80%, `gofmt`, e2e en vivo. | — |

## Risks

| Riesgo | Sev | Mitigación |
|--------|-----|-----------|
| Parseo de `actions`/`purchase_roas` (arrays, múltiples action_type) | ALTA | Fase 0 (sondeo en vivo) + tests con JSON real |
| Doble conteo de compras | MED | Regla de selección de `action_type` (decisión 1) |
| Combos de `breakdowns` inválidos | MED | Validación de enum + traducción del error de Meta |
| Cambio de retorno de `GetInsights` afecta la tool existente | MED | Actualizar presentación de la feature 001 en la misma entrega |

## Complexity Tracking

> Sin violaciones de la constitución. No aplica.
