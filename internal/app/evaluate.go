package app

import "github.com/mashats/meta-ads-manager/internal/domain"

// Evaluation es el resultado de comparar las métricas de una campaña/segmento
// contra los umbrales de negocio (Principio VIII), respetando la suficiencia de
// muestra (Principio IX). La capa de presentación lo traduce a íconos/español.
type Evaluation struct {
	ROAS         domain.MetricStatus
	CPA          domain.MetricStatus
	LinkCTR      domain.MetricStatus
	Frequency    domain.MetricStatus
	TicketUsed   float64 // ticket aplicado para comparar el CPA
	Insufficient bool    // muestra insuficiente para concluir sobre conversión
}

// Evaluate aplica umbrales y política de suficiencia a un conjunto de métricas.
// days es la cantidad de días del período (para la suficiencia temporal).
func Evaluate(m domain.Metrics, days int, th domain.Thresholds, su domain.SufficiencyPolicy) Evaluation {
	// ¿Hay suficiente muestra de conversión para concluir ROAS/CPA?
	convSufficient := m.Purchases != nil && *m.Purchases >= su.MinPurchases && days >= su.MinDays

	// ¿Hay suficiente entrega para evaluar tasas de enlace?
	rateSufficient := m.Impressions >= su.MinImpressions && m.LinkClicks >= su.MinLinkClicks

	ticket := effectiveTicket(m, th)

	return Evaluation{
		ROAS:         roasStatus(m, th, convSufficient),
		CPA:          cpaStatus(m, ticket, convSufficient),
		LinkCTR:      linkCTRStatus(m, th, rateSufficient),
		Frequency:    frequencyStatus(m, th, su),
		TicketUsed:   ticket,
		Insufficient: !convSufficient,
	}
}

// effectiveTicket deriva el ticket de facturación÷compras cuando hay datos; si
// no, cae al default de negocio (≈80.000 ARS), que siempre está disponible.
func effectiveTicket(m domain.Metrics, th domain.Thresholds) float64 {
	if m.Revenue != nil && m.Purchases != nil && *m.Purchases > 0 {
		return *m.Revenue / float64(*m.Purchases)
	}
	return th.AvgTicket
}

func roasStatus(m domain.Metrics, th domain.Thresholds, convSufficient bool) domain.MetricStatus {
	if m.ROAS == nil || !convSufficient {
		return domain.StatusNoData
	}
	if *m.ROAS >= th.MinROAS {
		return domain.StatusOK
	}
	return domain.StatusBad
}

func cpaStatus(m domain.Metrics, ticket float64, convSufficient bool) domain.MetricStatus {
	if m.CPA == nil || !convSufficient || ticket <= 0 {
		return domain.StatusNoData
	}
	if *m.CPA <= ticket {
		return domain.StatusOK
	}
	return domain.StatusBad
}

func linkCTRStatus(m domain.Metrics, th domain.Thresholds, rateSufficient bool) domain.MetricStatus {
	if !rateSufficient {
		return domain.StatusNoData
	}
	if m.LinkCTR >= th.LinkCTRMin && m.LinkCTR <= th.LinkCTRMax {
		return domain.StatusOK
	}
	return domain.StatusWarn
}

func frequencyStatus(m domain.Metrics, th domain.Thresholds, su domain.SufficiencyPolicy) domain.MetricStatus {
	if m.Impressions < su.MinImpressions {
		return domain.StatusNoData
	}
	if m.Frequency <= th.MaxFrequency {
		return domain.StatusOK
	}
	return domain.StatusWarn
}
