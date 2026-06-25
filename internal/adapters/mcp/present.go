package mcp

import (
	"fmt"
	"strings"

	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

// messageForError traduce un error semántico del dominio a un mensaje legible
// en español para el usuario final no técnico (Constitución, Principio VII).
// Nunca expone detalles técnicos, códigos ni stack traces.
func messageForError(err error) string {
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
