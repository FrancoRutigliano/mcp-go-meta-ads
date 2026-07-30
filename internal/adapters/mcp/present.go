package mcp

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

// messageForError traduce un error semántico del dominio a un mensaje legible
// en español para el usuario final no técnico (Constitución, Principio VII).
// Nunca expone detalles técnicos, códigos ni stack traces.
func messageForError(err error) string {
	// Primero las condiciones de negocio que necesitan un mensaje preciso, con
	// cifras reales cuando corresponde. Recién si ninguna aplica se cae al
	// switch genérico por Kind.
	if msg, ok := budgetMessage(err); ok {
		return msg
	}
	if msg, ok := proposalMessage(err); ok {
		return msg
	}

	switch domain.KindOf(err) {
	case domain.KindUnauthorized:
		return "No pude acceder a tu cuenta publicitaria: la credencial no es válida o no tiene permisos de lectura. Revisá el acceso e intentá de nuevo."
	case domain.KindRateLimited:
		return "Meta está limitando las consultas en este momento. Esperá unos instantes e intentá de nuevo."
	case domain.KindNotFound:
		return "No encontré la campaña o la cuenta indicada. Verificá que el identificador sea correcto."
	case domain.KindInvalidInput:
		return "Los datos del pedido no son válidos. Revisá el período (fechas) o los parámetros e intentá de nuevo."
	default:
		return "Hubo un problema al consultar Meta. Probá de nuevo en unos minutos; si el problema persiste, avisá al equipo."
	}
}

// budgetMessage resuelve los mensajes propios de la escritura de presupuesto
// (feature 010). Devuelve ok=false si el error no es de este dominio, para que
// el llamador caiga al mensaje genérico por Kind.
func budgetMessage(err error) (string, bool) {
	var ge *domain.GuardrailError
	if errors.As(err, &ge) {
		switch ge.Limit {
		case domain.LimitIncreaseFactor:
			return fmt.Sprintf(
				"No puedo subir el presupuesto de %s a %s de una sola vez: es un salto demasiado grande. "+
					"El máximo permitido en un solo cambio es %s. Si querés llegar más alto, hacelo en varios pasos.",
				formatMoney(ge.Current), formatMoney(ge.Attempted), formatMoney(ge.Max)), true
		case domain.LimitDailyCeiling:
			return fmt.Sprintf(
				"Un presupuesto de %s por día supera el tope de seguridad configurado, que es %s por día. "+
					"Si de verdad querés gastar más, hay que subir ese tope en la configuración del servidor.",
				formatMoney(ge.Attempted), formatMoney(ge.Max)), true
		}
	}

	// El nivel equivocado no es un callejón sin salida: cuando la plata está en
	// los conjuntos, se los mostramos para que elija (FR-008).
	var le *domain.BudgetLevelError
	if errors.As(err, &le) {
		return levelMismatchMessage(le), true
	}

	switch {
	case errors.Is(err, domain.ErrBudgetLevelMismatch):
		return "El presupuesto de esa campaña no se controla en el nivel que indicaste. " +
			"Fijate con la tool get_budgets si la plata está en la campaña o repartida en sus conjuntos de anuncios.", true
	case errors.Is(err, domain.ErrNoBudgetToScale):
		return "No puedo aplicar un porcentaje porque esa entidad no tiene un presupuesto vigente sobre el cual calcularlo. " +
			"Indicá un monto en pesos.", true
	case errors.Is(err, domain.ErrBothAmountAndPercent):
		return "Indicá una sola cosa: o el monto en pesos, o el porcentaje de ajuste. No las dos juntas.", true
	case errors.Is(err, domain.ErrNoBudgetChange):
		return "No me dijiste cuánto querés que quede el presupuesto. Indicá un monto en pesos o un porcentaje de ajuste.", true
	case errors.Is(err, domain.ErrBudgetUnchanged):
		return "Ese presupuesto ya tiene ese valor, así que no hay nada que cambiar.", true
	case errors.Is(err, domain.ErrBudgetDrifted):
		return "El presupuesto cambió desde que armé la propuesta, así que no la aplico sobre información vieja. " +
			"Pedime la propuesta de nuevo para ver el valor actual.", true
	case errors.Is(err, domain.ErrBudgetBelowMinimum):
		return "Meta rechazó ese monto porque es menor al presupuesto mínimo que permite. " +
			"Probá con un monto más alto.", true
	case errors.Is(err, domain.ErrNoAdSets):
		return "Esa campaña no tiene conjuntos de anuncios, así que no hay dónde ajustar el presupuesto.", true
	}

	return "", false
}

// proposalMessage resuelve los mensajes del ciclo de vida de una propuesta.
// Sin esto, una propuesta vencida caía en el mensaje genérico de datos inválidos
// ("revisá el período") y una inexistente en el de campaña no encontrada: los
// dos mandan a la usuaria a buscar el problema donde no está.
func proposalMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrCampaignAlreadyInState):
		return "Esa campaña ya está en ese estado, así que no hay nada que cambiar.", true
	case errors.Is(err, domain.ErrProposalExpired):
		return "Esa propuesta venció por seguridad: sólo vale unos minutos desde que la pedís. " +
			"Pedila de nuevo y confirmala enseguida.", true
	case errors.Is(err, domain.ErrProposalNotFound):
		return "No encontré esa propuesta. Puede que ya la hayas confirmado (cada una sirve una sola vez), " +
			"que haya vencido, o que el identificador esté mal. Pedí una propuesta nueva y confirmá esa.", true
	}
	return "", false
}

// levelMismatchMessage explica dónde vive realmente el presupuesto y, cuando
// está en los conjuntos, los lista para que el usuario elija sin salir de la
// conversación.
func levelMismatchMessage(le *domain.BudgetLevelError) string {
	if le.Expected == domain.LevelCampaign {
		return fmt.Sprintf(
			"En la campaña \"%s\" el presupuesto se maneja de forma centralizada, no conjunto por conjunto. "+
				"Para cambiarlo, pedí el ajuste sobre la campaña y no sobre un conjunto de anuncios.",
			le.CampaignName)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "La campaña \"%s\" no tiene un presupuesto propio: la plata está repartida entre sus conjuntos de anuncios.\n",
		le.CampaignName)
	b.WriteString("Decime en cuál querés hacer el cambio:\n")
	for _, set := range le.AdSets {
		fmt.Fprintf(&b, "• %s — %s — %s (id %s)\n",
			set.Name, statusES(set.Status), adSetBudgetText(set), set.ID)
	}
	b.WriteString("\nDespués volvé a pedir el cambio indicando también el adset_id.")
	return b.String()
}

// adSetBudgetText describe el presupuesto de un conjunto.
func adSetBudgetText(set domain.AdSet) string {
	if !set.ManagesOwnBudget() {
		return "sin presupuesto propio"
	}
	return fmt.Sprintf("%s %s", formatMoney(set.Budget.Amount), budgetTypeES(set.Budget.Type))
}

// formatBudgets presenta dónde vive el presupuesto de una campaña y cuánto hay
// en cada lugar (FR-009, FR-010).
func formatBudgets(ov app.BudgetOverview) string {
	var b strings.Builder

	if ov.Level == domain.LevelCampaign {
		fmt.Fprintf(&b, "Campaña \"%s\": el presupuesto se maneja a nivel campaña.\n", ov.CampaignName)
		fmt.Fprintf(&b, "• Presupuesto %s: %s\n", budgetTypeES(ov.Campaign.Type), formatMoney(ov.Campaign.Amount))
		if ov.TotalDaily.IsZero() {
			b.WriteString("• Hoy no está gastando (la campaña no está activa o el presupuesto no es diario).\n")
		} else {
			fmt.Fprintf(&b, "• Gasto diario comprometido: %s (unos %s por mes).\n",
				formatMoney(ov.TotalDaily), formatMoney(domain.Money{Cents: ov.TotalDaily.Cents * 30}))
		}
		b.WriteString("\nPara cambiarlo, usá propose_budget con el campaign_id.")
		return strings.TrimRight(b.String(), "\n")
	}

	if n := len(ov.AdSets); n == 1 {
		fmt.Fprintf(&b, "Campaña \"%s\": el presupuesto lo maneja su único conjunto de anuncios.\n",
			ov.CampaignName)
	} else {
		fmt.Fprintf(&b, "Campaña \"%s\": la plata está repartida entre %d conjuntos de anuncios.\n",
			ov.CampaignName, n)
	}
	for _, set := range ov.AdSets {
		fmt.Fprintf(&b, "• %s — %s — %s (id %s)\n",
			set.Name, statusES(set.Status), adSetBudgetText(set), set.ID)
	}
	if ov.TotalDaily.IsZero() {
		b.WriteString("\nHoy no hay gasto diario comprometido: no hay conjuntos activos con presupuesto diario.")
	} else {
		fmt.Fprintf(&b, "\nGasto diario comprometido: %s (unos %s por mes).",
			formatMoney(ov.TotalDaily), formatMoney(domain.Money{Cents: ov.TotalDaily.Cents * 30}))
	}
	b.WriteString("\nPara cambiar uno, usá propose_budget con el campaign_id y el adset_id.")
	return b.String()
}

// formatMoney presenta un monto en pesos con separador de miles local. El
// usuario nunca ve unidades internas de la API (FR-034).
func formatMoney(m domain.Money) string {
	pesos := int64(math.Round(m.Pesos()))
	sign := ""
	if pesos < 0 {
		sign = "-"
		pesos = -pesos
	}

	digits := strconv.FormatInt(pesos, 10)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}
	return sign + "$" + b.String()
}

// formatCampaigns arma un resumen legible de las campañas. truncated indica que
// hay más resultados de los devueltos (FR-012).
func formatCampaigns(campaigns []domain.Campaign, truncated bool) string {
	if len(campaigns) == 0 {
		return "No hay campañas que coincidan con el pedido en esta cuenta."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Se encontraron %d campañas:\n", len(campaigns))
	for _, c := range campaigns {
		fmt.Fprintf(&b, "• %s — %s — objetivo: %s (id %s)\n",
			c.Name, statusES(c.Status), objectiveES(c.Objective), c.ID)
	}
	if truncated {
		b.WriteString("\nHay más campañas de las mostradas. Pedí un límite mayor o filtrá para ver el resto.")
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatInsights arma un resumen legible del rendimiento evaluado, indicando el
// período aplicado (FR-004) y el estado de cada métrica vs umbral (Principio VIII).
func formatInsights(reports []app.CampaignInsight, applied domain.DateRange) string {
	period := periodES(applied)
	if len(reports) == 0 {
		return fmt.Sprintf("No hubo actividad registrada en el período %s.", period)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Rendimiento %s:\n", period)
	for _, r := range reports {
		fmt.Fprintf(&b, "• %s (id %s)\n", nameOr(r.Insight.CampaignName, r.Insight.CampaignID), r.Insight.CampaignID)
		b.WriteString(metricsReport(r.Insight.Metrics, r.Eval))
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatBreakdown arma un resumen legible del desglose por audiencia.
func formatBreakdown(br app.EvaluatedBreakdown, applied domain.DateRange) string {
	period := periodES(applied)
	if len(br.Segments) == 0 {
		return fmt.Sprintf("No hubo actividad segmentada por %s en el período %s.",
			dimensionES(br.Dimension), period)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Rendimiento por %s %s:\n", dimensionES(br.Dimension), period)
	for _, s := range br.Segments {
		label := s.Label
		if strings.TrimSpace(label) == "" {
			label = "(segmento sin dato)"
		}
		fmt.Fprintf(&b, "• %s\n", label)
		b.WriteString(metricsReport(s.Metrics, s.Eval))
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatProposal presenta una propuesta de escritura SIN aplicar, explicando
// cómo confirmarla (Principio II: propose no tiene efecto).
func formatProposal(p domain.Proposal) string {
	before := statusES(domain.CampaignStatus(p.Before))
	after := statusES(domain.CampaignStatus(p.After))
	return fmt.Sprintf(
		"Propuesta lista (todavía no cambié nada):\n"+
			"• Acción: %s la campaña \"%s\" — de %s a %s.\n"+
			"• Para aplicarlo, confirmá con la tool confirm_action usando proposal_id=%s\n"+
			"• La propuesta vence a las %s.",
		accionES(p.After), nameOr(p.CampaignName, p.CampaignID), before, after,
		p.ID, p.ExpiresAt.Format("15:04"),
	)
}

// formatBudgetProposal presenta el cambio de presupuesto con el monto actual, el
// resultante, la variación, la proyección mensual y el rendimiento reciente,
// para que el usuario decida informado antes de confirmar (FR-003, FR-004,
// FR-019, FR-023 a FR-025).
func formatBudgetProposal(bp app.BudgetProposal) string {
	p := bp.Proposal
	d := p.Budget
	var b strings.Builder

	b.WriteString("Propuesta lista (todavía no cambié nada):\n")
	fmt.Fprintf(&b, "• %s: \"%s\"%s\n", nivelES(p.Level), entityNameOf(p), campaignSuffix(p))
	fmt.Fprintf(&b, "• Presupuesto %s: %s → %s (%s)\n",
		budgetTypeES(d.Type), formatMoney(d.Before), formatMoney(d.After), variacionES(d.Before, d.After))

	if d.Type == domain.BudgetDaily {
		fmt.Fprintf(&b, "• Eso son unos %s por mes si se mantiene todo el mes.\n",
			formatMoney(domain.Money{Cents: d.After.Cents * 30}))
	}

	if p.Active {
		b.WriteString("• Está activa: el cambio empieza a gastar apenas lo confirmes.\n")
	} else {
		b.WriteString("• Está pausada: el cambio queda guardado pero no genera gasto hasta que la actives.\n")
	}

	b.WriteString(perfBlock(bp))

	fmt.Fprintf(&b, "• Para aplicarlo, confirmá con la tool confirm_action usando proposal_id=%s\n", p.ID)
	fmt.Fprintf(&b, "• La propuesta vence a las %s.", p.ExpiresAt.Format("15:04"))

	return b.String()
}

// perfBlock arma el bloque de rendimiento que acompaña a la propuesta. Es
// honesto en los dos sentidos: si no hay datos suficientes lo dice, y si no se
// pudo leer el rendimiento tampoco lo oculta (Principios VIII y IX).
func perfBlock(bp app.BudgetProposal) string {
	if bp.PerfErr {
		return "• No pude leer el rendimiento reciente de esta campaña, así que estarías decidiendo sin ese dato.\n"
	}
	if bp.Metrics == nil {
		return fmt.Sprintf("• Sin actividad registrada %s: no hay rendimiento sobre el cual apoyar esta decisión.\n",
			periodES(bp.Period))
	}

	var b strings.Builder
	m := bp.Metrics
	fmt.Fprintf(&b, "• Rendimiento %s: ROAS %s · gasto %s · compras %s\n",
		periodES(bp.Period), roasText(*m, bp.Eval), formatMoney(domain.MoneyFromPesos(m.Spend)), purchasesText(*m))

	switch {
	case bp.Eval.Insufficient:
		b.WriteString("  ⚠️ Son muy pocos datos para afirmar que convenga este cambio. Podés confirmarlo igual, pero es una apuesta.\n")
	case bp.Eval.ROAS == domain.StatusBad:
		b.WriteString("  ❌ Esta campaña rinde por debajo del mínimo de 2x. Subirle el presupuesto agranda la pérdida.\n")
	case bp.Eval.ROAS == domain.StatusOK:
		b.WriteString("  ✅ Rinde por encima del mínimo de 2x.\n")
	}
	return b.String()
}

// formatConfirmation presenta el resultado de una escritura ya aplicada.
func formatConfirmation(p domain.Proposal) string {
	if p.Kind == domain.ProposalBudget && p.Budget != nil {
		return fmt.Sprintf("✅ Hecho. El presupuesto %s de %s \"%s\" pasó de %s a %s.",
			budgetTypeES(p.Budget.Type), nivelES(p.Level), entityNameOf(p),
			formatMoney(p.Budget.Before), formatMoney(p.Budget.After))
	}
	return fmt.Sprintf("✅ Hecho. La campaña \"%s\" pasó de %s a %s.",
		nameOr(p.CampaignName, p.CampaignID),
		statusES(domain.CampaignStatus(p.Before)),
		statusES(domain.CampaignStatus(p.After)))
}

// variacionES describe el salto en porcentaje, con signo.
func variacionES(before, after domain.Money) string {
	if before.IsZero() {
		return "nuevo"
	}
	pct := (float64(after.Cents)/float64(before.Cents) - 1) * 100
	if pct >= 0 {
		return fmt.Sprintf("+%.0f%%", pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}

func nivelES(l domain.BudgetLevel) string {
	if l == domain.LevelAdSet {
		return "conjunto de anuncios"
	}
	return "campaña"
}

func budgetTypeES(t domain.BudgetType) string {
	if t == domain.BudgetLifetime {
		return "total"
	}
	return "diario"
}

// entityNameOf devuelve el nombre de la entidad afectada, con fallback al id.
func entityNameOf(p domain.Proposal) string {
	if strings.TrimSpace(p.EntityName) != "" {
		return p.EntityName
	}
	if p.EntityID != "" {
		return p.EntityID
	}
	return nameOr(p.CampaignName, p.CampaignID)
}

// campaignSuffix aclara a qué campaña pertenece un conjunto de anuncios.
func campaignSuffix(p domain.Proposal) string {
	if p.Level != domain.LevelAdSet {
		return ""
	}
	return fmt.Sprintf(" (de la campaña \"%s\")", nameOr(p.CampaignName, p.CampaignID))
}

// accionES traduce el estado destino a un verbo de acción.
func accionES(after string) string {
	if domain.CampaignStatus(after) == domain.CampaignActive {
		return "activar"
	}
	return "pausar"
}

// formatAdPerformance arma el ranking de anuncios (mejor a peor), indicando el
// período aplicado y el estado de cada métrica vs umbral.
func formatAdPerformance(reports []app.AdReport, applied domain.DateRange) string {
	period := periodES(applied)
	if len(reports) == 0 {
		return fmt.Sprintf("No hubo anuncios con actividad en el período %s.", period)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Rendimiento por anuncio %s (de mejor a peor):\n", period)
	for _, r := range reports {
		name := nameOr(r.Ad.AdName, r.Ad.AdID)
		if strings.TrimSpace(r.Ad.CampaignName) != "" {
			fmt.Fprintf(&b, "• %s — campaña: %s\n", name, r.Ad.CampaignName)
		} else {
			fmt.Fprintf(&b, "• %s\n", name)
		}
		b.WriteString(metricsReport(r.Ad.Metrics, r.Eval))
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatFunnel arma el embudo de conversión, mostrando la retención en cada
// paso y resaltando dónde se cae (responde "¿por qué no vende?").
func formatFunnel(funnels []app.CampaignFunnel, applied domain.DateRange) string {
	period := periodES(applied)
	if len(funnels) == 0 {
		return fmt.Sprintf("No hubo actividad registrada en el período %s.", period)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Embudo de conversión %s:\n", period)
	for _, f := range funnels {
		fmt.Fprintf(&b, "• %s (id %s)\n", nameOr(f.CampaignName, f.CampaignID), f.CampaignID)
		if !f.AnyActivity {
			b.WriteString("   Sin actividad en el período.\n")
			continue
		}

		var prevName string
		var prevCount int64
		for _, s := range f.Steps {
			if !s.Known {
				continue
			}
			if prevName == "" {
				fmt.Fprintf(&b, "   %s: %d\n", s.Name, s.Count)
			} else {
				ret := 0.0
				if prevCount > 0 {
					ret = float64(s.Count) / float64(prevCount) * 100
				}
				fmt.Fprintf(&b, "   %s: %d (%.1f%% de %s)\n", s.Name, s.Count, ret, strings.ToLower(prevName))
			}
			prevName, prevCount = s.Name, s.Count
		}

		if f.HasLeak {
			fmt.Fprintf(&b, "   🔴 Mayor caída: de \"%s\" a \"%s\" (se pierde %.1f%%).\n",
				f.LeakFrom, f.LeakTo, f.LeakPct)
			if hint := leakHint(f.LeakTo); hint != "" {
				fmt.Fprintf(&b, "   → %s\n", hint)
			}
		}
		if f.PixelGaps {
			b.WriteString("   ℹ️ El pixel no registra algunos pasos intermedios (carrito/pago); el embudo puede estar incompleto.\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// leakHint traduce dónde se cae el embudo a una explicación accionable.
func leakHint(leakTo string) string {
	switch leakTo {
	case "Vistas de página":
		return "Hacen clic pero no llegan a la web: revisá velocidad de carga o que el enlace no esté roto."
	case "Agregar al carrito":
		return "Llegan a la web pero no agregan al carrito: el producto, la foto o el precio no enganchan."
	case "Iniciar pago":
		return "Agregan al carrito pero no arrancan el pago: puede ser el costo de envío o dudas antes de pagar."
	case "Compras":
		return "Arrancan el pago pero no compran: revisá el checkout, envío y medios de pago en Tienda Nube."
	default:
		return ""
	}
}

// metricsReport renderiza el bloque de métricas de un conjunto, con íconos por
// estado y "no calculable" cuando falta el dato (Principios VIII y IX).
func metricsReport(m domain.Metrics, e app.Evaluation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "   gasto: $%.2f ARS · impresiones: %d · clics: %d · alcance: %d\n",
		m.Spend, m.Impressions, m.Clicks, m.Reach)
	fmt.Fprintf(&b, "   ROAS: %s · compras: %s · facturación: %s\n",
		roasText(m, e), purchasesText(m), revenueText(m))
	fmt.Fprintf(&b, "   CPA: %s · CTR enlace: %s · frecuencia: %s\n",
		cpaText(m, e), linkCTRText(m, e), freqText(m, e))
	if e.Insufficient {
		b.WriteString("   ⚠️ Datos insuficientes para recomendar apagar o prender esta campaña.\n")
	}
	return b.String()
}

func periodES(r domain.DateRange) string {
	return fmt.Sprintf("del %s al %s", r.Since.Format("02/01/2006"), r.Until.Format("02/01/2006"))
}

func icon(s domain.MetricStatus) string {
	switch s {
	case domain.StatusOK:
		return "✅"
	case domain.StatusWarn:
		return "⚠️"
	case domain.StatusBad:
		return "❌"
	default:
		return "•"
	}
}

func roasText(m domain.Metrics, e app.Evaluation) string {
	if m.ROAS == nil {
		return "no calculable (sin conversiones)"
	}
	if e.ROAS == domain.StatusNoData {
		return fmt.Sprintf("%.2fx (datos insuficientes)", *m.ROAS)
	}
	return fmt.Sprintf("%s %.2fx", icon(e.ROAS), *m.ROAS)
}

func cpaText(m domain.Metrics, e app.Evaluation) string {
	if m.CPA == nil {
		return "no calculable (sin conversiones)"
	}
	if e.CPA == domain.StatusNoData {
		return fmt.Sprintf("$%.2f ARS (datos insuficientes)", *m.CPA)
	}
	return fmt.Sprintf("%s $%.2f ARS (ticket ref $%.0f)", icon(e.CPA), *m.CPA, e.TicketUsed)
}

func purchasesText(m domain.Metrics) string {
	if m.Purchases == nil {
		return "sin datos"
	}
	return fmt.Sprintf("%d", *m.Purchases)
}

func revenueText(m domain.Metrics) string {
	if m.Revenue == nil {
		return "sin datos"
	}
	return fmt.Sprintf("$%.2f ARS", *m.Revenue)
}

func linkCTRText(m domain.Metrics, e app.Evaluation) string {
	if e.LinkCTR == domain.StatusNoData {
		return "sin datos suficientes"
	}
	return fmt.Sprintf("%s %.2f%%", icon(e.LinkCTR), m.LinkCTR)
}

func freqText(m domain.Metrics, e app.Evaluation) string {
	if e.Frequency == domain.StatusNoData {
		return "sin datos"
	}
	return fmt.Sprintf("%s %.2f", icon(e.Frequency), m.Frequency)
}

func dimensionES(d domain.BreakdownDimension) string {
	switch d {
	case domain.DimensionAge:
		return "edad"
	case domain.DimensionGender:
		return "género"
	case domain.DimensionRegion:
		return "región"
	case domain.DimensionPublisherPlatform:
		return "plataforma"
	case domain.DimensionPlatformPosition:
		return "posición del anuncio"
	default:
		return string(d)
	}
}

func nameOr(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return "Campaña " + id
}

func statusES(s domain.CampaignStatus) string {
	switch s {
	case domain.CampaignActive:
		return "activa"
	case domain.CampaignPaused:
		return "pausada"
	case domain.CampaignArchived:
		return "archivada"
	case domain.CampaignDeleted:
		return "eliminada"
	default:
		return string(s)
	}
}

func objectiveES(o string) string {
	switch o {
	case "OUTCOME_SALES":
		return "Ventas"
	case "MESSAGES":
		return "Mensajes"
	case "LINK_CLICKS":
		return "Clics en enlace"
	case "OUTCOME_TRAFFIC":
		return "Tráfico"
	case "OUTCOME_ENGAGEMENT":
		return "Interacción"
	case "OUTCOME_LEADS":
		return "Clientes potenciales"
	case "OUTCOME_AWARENESS":
		return "Reconocimiento"
	default:
		return o
	}
}
