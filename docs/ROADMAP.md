# Roadmap — meta-ads-manager

Servidor MCP para que un usuario **no técnico** (Mariana, Mas Hats) entienda y gestione sus
campañas de Meta Ads hablando en español.

## Lo que la clienta va a querer el día 1
- "¿Por qué no vende lo que hago?" → diagnóstico (embudo, qué se cae).
- "¿Qué públicos funcionan y cuáles no?" → análisis por audiencia / ad set.
- "¿Qué anuncios/campañas funcionan?" → rendimiento por campaña **y por anuncio (creativo)**.
- "Poné la plata en lo que funciona." → priorizar presupuesto (ESCRITURA).
- "Quiero probar cosas nuevas." → crear campañas/públicos (ESCRITURA).
- Estacionalidad (verano) → comparar períodos, ver tendencia.
- Necesita **recomendaciones**, no números crudos.

---

## Estado

### ✅ Hecho y validado en vivo
- **001 — Lectura de campañas e insights** (`get_campaigns`, `get_campaigns_insights`).
- **002 — Insights de decisión** (ROAS, CPA, compras, facturación, CTR de enlace, frecuencia +
  `get_audience_breakdown` por edad/género/región/plataforma/posición). Validado con datos reales:
  ROAS 6,39x, ticket derivado ~$79.871, Principio IX honesto ante segmentos sin conversión.

### 🔜 Lectura (rápido, seguro, alto valor)
- ✅ **003 — Fix umbral CTR de enlace**: alerta sólo cuando es **bajo** (<0,8%); no hay techo. Se
  eliminó `LINK_CTR_MAX`. **Hecho — falta redeploy.**
- ✅ **004 — Embudo de conversión** (`get_conversion_funnel`): impresiones → clic → vistas → carrito
  → inicio de pago → compras, resaltando dónde se cae + pista accionable. Pasos intermedios
  nullables (Principio IX). **Hecho — falta redeploy.**
- ✅ **005 — Rendimiento por anuncio (creativo)** (`get_ad_performance`): rendimiento por anuncio
  (level=ad), evaluado y ordenado de mejor a peor por ROAS. Responde "¿qué anuncio funciona?".
  **Hecho — falta redeploy.**
- **006 — Ranking y comparación de períodos**: top ganadoras vs perdedoras; este verano vs anterior
  (estacionalidad).
- **007 — Descubrimiento de públicos (lectura)**: nivel **ad set**, leer targeting actual, buscar
  intereses/"gustos" + estimador de tamaño de audiencia.
- **Capa de interpretación/recomendación** (skill `meta-ads-interpretacion`): convertir números en
  "esto anda, poné plata; esto no, apagá; 65+ no compra, sacalo". Transversal.

### 🔒 Seguridad / cimiento (precondición de escritura)
- ✅ **008 — Auth del endpoint MCP** (bearer `MCP_AUTH_TOKEN`): middleware con comparación en
  tiempo constante. Si la var no está seteada, queda abierto y lo advierte en el log.
  **Hecho — falta setear `MCP_AUTH_TOKEN` en Railway y re-conectar el cliente con el header.**
- ✅ **009 — Cimiento de escritura**: flujo **propose/confirm** (Principio II) + **auditoría**
  estructurada (Principio IV) + puerto `MetaWriter` separado + store de propuestas en memoria.
  Primera operación real incluida: **pausar/activar campaña** (`propose_campaign_status` +
  `confirm_action`). Ninguna escritura sin propose previo válido.
  **Hecho — falta redeploy + token `ads_management` para que la escritura funcione.**

### ✍️ Escritura (después de 008 + 009)
- ✅ **010 — Ajustar presupuesto**: `get_budgets` (dónde vive la plata y cuánta hay) +
  `propose_budget` sobre el cimiento propose/confirm. Cubre **campaña y conjunto de anuncios**:
  si la campaña reparte el presupuesto, la propuesta lista los conjuntos para elegir. Acepta
  monto absoluto y ajuste porcentual (el porcentaje se resuelve en el servidor y se confirma en
  pesos). Dos guardrails configurables: factor máximo de aumento (3x) y techo diario
  (`BUDGET_MAX_DAILY_ARS`). Detecta deriva entre propose y confirm. La propuesta muestra ROAS
  evaluado contra 2x y advierte cuando no hay datos suficientes, sin bloquear.
  **Hecho y validado en vivo (29/07/2026): el presupuesto de esta cuenta vive a nivel ad set**, no
  campaña. Factor de moneda ARS confirmado (100). Escritura probada de punta a punta sobre un ad
  set pausado (ida y vuelta, gasto $0). Falta redeploy.
- **011 — Gestión de públicos (escritura)**: crear Custom/Lookalike/Saved audiences, editar
  targeting de un ad set. propose/confirm. ⚠️ Custom Audiences desde lista de clientes toca PII.

---

## Verificación en vivo — 29/07/2026

Las 9 tools probadas contra la cuenta real (`act_331498724`). Todas funcionan. Se encontraron y
arreglaron 4 defectos; el primero era crítico.

| # | Defecto | Cómo se veía |
|---|---------|--------------|
| 1 | **Token de Meta filtrado a los logs** | Ante una falla de transporte, el `*url.Error` de net/http traía la URL completa con el token en la query, y se logueaba entero. En Railway = credencial en texto plano en el stream de logs. |
| 2 | CTR alto marcado como alerta | 4,90% con ⚠️ (roadmap 003) |
| 3 | `platform_position` fallaba siempre | Meta rechaza `(action_type, platform_position)`; hay que pedirla junto con `publisher_platform` |
| 4 | Mensajes genéricos engañosos | Propuesta vencida → "revisá el período (fechas)" |

Además: timeout del cliente HTTP 30s → 120s (los insights de rangos largos fallaban).

Verificado del camino de escritura: `propose` no escribe; `confirm` sí; propuesta de **un solo uso**;
**vencimiento a los 5 min**; **detección de deriva** (se modificó el presupuesto por detrás en Meta y
el confirm se negó a aplicar); los dos guardrails frenan por separado; auditoría completa y sin el
token. Cuenta restaurada a sus valores originales, 0 campañas activas.

### Pendiente de esta verificación
- **Rotar el token de Meta**: estuvo en logs locales por el defecto 1.
- Ventana por defecto de 30 días: la cuenta no gasta desde junio, así que "¿cómo va mi publicidad?"
  responde "sin datos". Es honesto (Principio IX) pero puede confundir. Evaluar cambiar el default.
- `MIN_PURCHASES=1` es demasiado bajo: un anuncio con 1 compra sale con ROAS 30,21x y primero en el
  ranking. Subirlo a 3-5 por env var (decisión de negocio, no requiere código).

---

## Notas de riesgo / realidad
- **Pixel: confirmado que atribuye** (vimos 22 compras reales atribuidas). Gran riesgo despejado.
- Meta **deprecó Audience Insights** (~2021): el "¿qué le gusta a mi cliente?" masivo es limitado;
  la búsqueda de intereses y el estimador de tamaño sí funcionan.
- La campaña de Ventas rendía 6,39x y **está en $0** hace semanas: reactivarla es acción de
  escritura (010). Confirmado el 29/07: **toda la cuenta está pausada** y el último gasto fue en
  junio. Mayo–junio 2026 rindió ROAS 6,96x (Frios) y 5,73x (Cálidos/Mixtos).
- Todo lo de escritura mueve gasto real e irreversible → propose/confirm no es opcional.
