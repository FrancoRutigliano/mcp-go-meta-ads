# meta-ads-manager

Servidor **MCP** (Model Context Protocol) en Go para gestionar campañas de **Meta Ads**.
Primera entrega: **lectura** de campañas y de su rendimiento. El servidor es la única
pieza que habla con la Graph API de Meta; el asistente (Claude) nunca ve el token.

## Tools expuestas

| Tool | Qué hace |
|------|----------|
| `get_campaigns` | Lista campañas de la cuenta (por defecto sólo activas). Parámetros: `status` (`active`\|`all`), `limit`. |
| `get_campaigns_insights` | Rendimiento por campaña (gasto, impresiones, clics, alcance, CTR, CPC) para un período. Parámetros: `campaign_id` (opcional), `since`/`until` en `AAAA-MM-DD` (opcionales; por defecto últimos 30 días). |

Ambas son de **solo lectura**: no modifican gasto ni estado de campañas.

## Arquitectura (hexagonal)

```text
cmd/server            arranque + transporte HTTP
internal/
  config              carga y valida el entorno (fail-fast, token redactado)
  domain              entidades y errores semánticos (sin dependencias externas)
  ports               interfaz MetaReader (puerto del dominio)
  app                 casos de uso (ListCampaigns, GetInsights)
  adapters/meta       cliente Graph API: único que conoce el token (salida)
  adapters/mcp        tools MCP + presentación en español (entrada)
```

## Configuración

Variables de entorno (ver `.env.example`):

| Variable | Obligatoria | Default | Descripción |
|----------|:-----------:|---------|-------------|
| `META_TOKEN` | sí | — | Token de acceso de Meta. Necesita `ads_management` para las operaciones de escritura. |
| `META_AD_ACCOUNT_ID` | sí | — | Cuenta objetivo con prefijo `act_`. |
| `META_API_VERSION` | no | `v21.0` | Versión de la Graph API. |
| `PORT` | no | `8080` | Puerto del transporte Streamable HTTP. |
| `MCP_AUTH_TOKEN` | no | — | Bearer que protege el endpoint MCP. Si está vacío, el endpoint queda abierto y se advierte en el log. |

### Límites de seguridad de escritura

Acotan cuánto puede crecer el gasto en una sola operación de presupuesto. El primero frena el
error de tipeo puntual (agregar un cero); el segundo, el escalamiento acumulado por cambios
sucesivos.

| Variable | Obligatoria | Default | Descripción |
|----------|:-----------:|---------|-------------|
| `BUDGET_MAX_INCREASE_FACTOR` | no | `3` | Múltiplo máximo del presupuesto vigente en un solo cambio. Debe ser ≥ 1. |
| `BUDGET_MAX_DAILY_ARS` | no | `25000` | Techo absoluto de presupuesto diario, en pesos. Debe ser > 0. |

### Umbrales de negocio

| Variable | Default | Descripción |
|----------|---------|-------------|
| `ROAS_MIN` | `2` | ROAS mínimo aceptable. |
| `LINK_CTR_MIN` / `LINK_CTR_MAX` | `0.8` / `1.5` | Rango saludable de CTR de enlace (%). |
| `FREQUENCY_MAX` | `3.5` | Frecuencia a partir de la cual se alerta fatiga. |
| `AVG_TICKET` | — | Ticket promedio de referencia para evaluar el CPA. |
| `MIN_PURCHASES` / `MIN_IMPRESSIONS` / `MIN_LINK_CLICKS` / `MIN_DAYS` | `1` / `1000` / `50` / `7` | Mínimos de muestra para sostener una conclusión. |

> El token sólo se lee del entorno y nunca se escribe en logs ni respuestas. Si falta el
> token o la cuenta, el servidor **aborta el arranque** con un mensaje claro. Un guardrail mal
> configurado también aborta el arranque: es peor una falsa sensación de protección que ninguna.

## Uso

```bash
# Local (carga .env automáticamente)
make run

# Tests y cobertura
make test
make cover

# Imagen Docker (binario estático sobre distroless)
make docker
```

El servidor queda escuchando en `:$PORT` con el endpoint MCP en `/mcp`.

## Seguridad

Este proyecto sigue una constitución (`.specify/memory/constitution.md`). Puntos clave de
esta feature: token confinado al entorno, errores **semánticos nunca silenciados**, mensajes
de usuario en **español sin detalles técnicos**, y **rate limiting** con backoff hacia Meta.
