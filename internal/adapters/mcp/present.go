package mcp

import (
	"fmt"
	"strings"

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

// formatInsights arma un resumen legible del rendimiento, indicando el período
// aplicado de forma explícita (FR-004).
func formatInsights(insights []domain.Insight, applied domain.DateRange) string {
	period := fmt.Sprintf("del %s al %s",
		applied.Since.Format("02/01/2006"), applied.Until.Format("02/01/2006"))

	if len(insights) == 0 {
		return fmt.Sprintf("No hubo actividad registrada en el período %s.", period)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Rendimiento %s:\n", period)
	for _, in := range insights {
		m := in.Metrics
		fmt.Fprintf(&b,
			"• %s — gasto: $%.2f ARS · impresiones: %d · clics: %d · alcance: %d · CTR: %.2f%% · CPC: $%.2f\n",
			nameOr(in.CampaignName, in.CampaignID), m.Spend, m.Impressions, m.Clicks, m.Reach, m.CTR, m.CPC)
	}
	return strings.TrimRight(b.String(), "\n")
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
