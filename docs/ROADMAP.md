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
- **003 — Fix umbral CTR de enlace**: alertar solo cuando es **bajo** (<0,8%); un CTR alto es bueno.
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
  **Hecho — falta redeploy + validar contra la cuenta real en qué nivel está el presupuesto.**
- **011 — Gestión de públicos (escritura)**: crear Custom/Lookalike/Saved audiences, editar
  targeting de un ad set. propose/confirm. ⚠️ Custom Audiences desde lista de clientes toca PII.

---

## Notas de riesgo / realidad
- **Pixel: confirmado que atribuye** (vimos 22 compras reales atribuidas). Gran riesgo despejado.
- Meta **deprecó Audience Insights** (~2021): el "¿qué le gusta a mi cliente?" masivo es limitado;
  la búsqueda de intereses y el estimador de tamaño sí funcionan.
- La campaña de Ventas rendía 6,39x y **está en $0** hace semanas: reactivarla es acción de
  escritura (010).
- Todo lo de escritura mueve gasto real e irreversible → propose/confirm no es opcional.
