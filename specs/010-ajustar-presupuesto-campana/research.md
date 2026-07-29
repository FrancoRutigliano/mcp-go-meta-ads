# Fase 0 — Investigación: presupuestos en la Meta Graph API

**Feature**: 010 — Ajustar presupuesto de campañas y conjuntos de anuncios
**Fecha**: 2026-07-28
**Estado**: parcialmente resuelto — ver "Incógnitas abiertas"

Este documento existe porque el riesgo más alto del plan es escribir un monto mal interpretado sobre
gasto real. Cada afirmación va con su nivel de confianza; lo que no está confirmado se marca como
tal en lugar de asumirse (Principio IX aplicado a nuestro propio proceso).

---

## R1 — ¿Cómo devuelve Meta el presupuesto?

**Confianza: alta.**

Los objetos `Campaign` y `AdSet` exponen `daily_budget` y `lifetime_budget` como **strings en
unidades menores** de la moneda de la cuenta. Para ARS el factor es 100, o sea que `"150000"`
significa $1.500,00 ARS por día.

**Decisión**: `domain.Money{Cents int64}` con parseo desde string. El factor 100 se define como
constante `arsMinorUnits` en el adaptador, no se esparce por el código.

**Riesgo residual**: el factor de unidades menores es una propiedad de la moneda de la cuenta
(`currency_offset`), no una constante universal — hay monedas con factor 1 (JPY, CLP). Como la
constitución fija ARS y el alcance es una sola cuenta, se hardcodea 100 **con un comentario que lo
explicita**. Si alguna vez se soporta otra cuenta, hay que leer el offset de la cuenta.

---

## R2 — ¿Cómo se distingue "presupuesto en la campaña" de "presupuesto en los conjuntos"?

**Confianza: alta.**

El presupuesto a nivel campaña es lo que Meta llama *Advantage campaign budget* (antes CBO). Si la
campaña tiene `daily_budget` o `lifetime_budget` con valor, la plata se administra ahí y los
conjuntos **no** tienen presupuesto propio. Si la campaña no los trae, cada conjunto administra el
suyo. Los dos niveles son mutuamente excluyentes dentro de una misma campaña.

**Decisión**: `Campaign.Budget == nil` ⇒ el presupuesto vive en los conjuntos (D2 del plan). No hace
falta un flag adicional.

**Riesgo residual**: Meta puede devolver el campo ausente o en `"0"`. El mapper debe tratar **ambos**
casos como "sin presupuesto a nivel campaña". Hay un test explícito para esto.

---

## R3 — ¿Cómo se escribe un presupuesto?

**Confianza: alta.**

`POST /{campaign-id}` con `daily_budget=<unidades menores>`, y análogamente `POST /{adset-id}`. Es el
mismo patrón que ya usa `UpdateCampaignStatus` (`client.go:173`), así que se reutiliza `c.post()` sin
cambios en el transporte.

**Decisión**: `UpdateCampaignBudget` y `UpdateAdSetBudget` como dos métodos explícitos del puerto de
escritura, cada uno mandando el campo que corresponda al tipo de presupuesto (`daily_budget` o
`lifetime_budget`).

---

## R4 — ¿Qué pasa si se escribe en el nivel equivocado?

**Confianza: media.**

Meta rechaza el intento de fijar un presupuesto de campaña cuando los conjuntos tienen el suyo (y
viceversa), pero **el código de error exacto no está confirmado**.

**Decisión**: no dependemos de ese error. El paso `propose` detecta el nivel leyendo la campaña
(R2) y rechaza antes de llegar a Meta (FR-007). El error de la API queda como red de contención, no
como mecanismo primario.

---

## R5 — Presupuesto mínimo: ¿qué error devuelve Meta?

**Confianza: baja. INCÓGNITA ABIERTA.**

Meta impone un presupuesto diario mínimo que depende de la moneda y del objetivo/optimización de la
campaña. No tengo confirmado el par código/subcódigo exacto que devuelve al rechazarlo, y **no se
va a inventar uno**.

**Resolución implementada (fallback, no cierre definitivo)**: `translateError` detecta la condición
por el **texto del mensaje** de Meta (`budget` + `minimum`/`at least`/`too low`) y la mapea a
`domain.ErrBudgetBelowMinimum` con `KindInvalidInput`. La presentación explica que Meta rechazó el
monto por ser menor al mínimo permitido y sugiere subirlo, **sin afirmar cuál es ese mínimo** porque
no lo sabemos.

Esta es la misma técnica que el código ya usaba para los errores de `breakdown`
(`mentionsBreakdown`), así que no introduce un patrón nuevo. Está cubierta por dos tests: uno que
verifica la detección y otro que verifica que un error que menciona "budget" por otro motivo **no**
se malinterprete.

**Limitación conocida**: la detección por texto es frágil ante cambios de redacción de Meta y ante
respuestas localizadas. No es equivalente a mapear por código.

**Cómo cerrarla de verdad**: intentar un `POST` con un monto deliberadamente bajo (ej. $1) sobre una
campaña **pausada** y capturar el par código/subcódigo real. Es una escritura real pero inocua sobre
una campaña sin gasto; aun así debe hacerse a conciencia y no como parte de la suite automatizada.
Cuando se obtenga, reemplazar la detección por texto por el mapeo en `kindFromCode`.

---

## R6 — ¿Se puede cambiar de presupuesto diario a total?

**Confianza: media.**

Cambiar el tipo de presupuesto de una campaña o conjunto ya en marcha tiene restricciones (y para
presupuesto total, interacción con las fechas de la campaña).

**Decisión**: fuera de alcance por spec (sección Assumptions). La feature ajusta el **monto** del
tipo que la entidad ya tiene. Si la entidad usa presupuesto total, se ajusta el total y la
presentación aclara que no es un monto diario.

---

## R7 — Campos a pedir en cada consulta

**Confianza: alta.**

| Consulta | Campos |
|---|---|
| Campaña (existente + nuevos) | `id,name,status,objective,daily_budget,lifetime_budget` |
| Conjuntos de una campaña | `id,name,status,campaign_id,daily_budget,lifetime_budget` |
| Endpoint de conjuntos | `GET /{campaign-id}/adsets` |

**Decisión**: extender la constante `campaignFields` en `client.go:27` y agregar `adSetFields`.

**Efecto colateral a vigilar**: `campaignFields` la usan `ListCampaigns` y `GetCampaign`. Agregar
campos cambia el payload de `get_campaigns`; los tests existentes del mapper deben seguir pasando
porque los campos nuevos son opcionales, pero hay que verificarlo explícitamente.

---

## Incógnitas abiertas

| # | Incógnita | Bloquea | Plan de cierre |
|---|---|---|---|
| R5 | Código de error de presupuesto por debajo del mínimo | No — fallback implementado por texto, con tests | Prueba en vivo controlada para obtener el subcódigo y reemplazar la detección por texto |
| R1b | Confirmar que la cuenta real usa factor 100 | No — es lo esperado para ARS | Leer `currency` y `currency_offset` de la cuenta en la primera prueba en vivo |
| R2b | Confirmar en cuál de los dos niveles está el presupuesto de las campañas reales | No — ambos están cubiertos | Correr `get_budgets` apenas exista (Fase 5) |

Ninguna incógnita abierta bloquea el arranque de la Fase 1 (dominio), que es pura y no depende del
formato de la API. R1 y R7 sí condicionan la Fase 2 y están en confianza alta.
