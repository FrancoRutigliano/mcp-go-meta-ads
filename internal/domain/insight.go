package domain

import (
	"errors"
	"time"
)

// DateRange es el período de un insight, inclusivo en ambos extremos.
type DateRange struct {
	Since time.Time
	Until time.Time
}

// Valid verifica que el rango sea utilizable: ambos extremos presentes y
// Since no posterior a Until.
func (r DateRange) Valid() error {
	if r.Since.IsZero() || r.Until.IsZero() {
		return errors.New("el período requiere fecha de inicio y de fin")
	}
	if r.Since.After(r.Until) {
		return errors.New("la fecha de inicio no puede ser posterior a la de fin")
	}
	return nil
}

// Metrics son las métricas de rendimiento que esta feature expone.
type Metrics struct {
	Spend       float64 // gasto en la moneda de la cuenta
	Impressions int64
	Clicks      int64
	Reach       int64
	CTR         float64 // click-through rate (%)
	CPC         float64 // costo por clic
}

// Insight es el rendimiento de una campaña en un período dado.
type Insight struct {
	CampaignID   string
	CampaignName string
	Range        DateRange
	Metrics      Metrics
}

// InsightQuery parametriza la lectura de rendimiento. CampaignID vacío indica
// rendimiento de todas las campañas de la cuenta (nivel campaña).
type InsightQuery struct {
	CampaignID string
	Range      DateRange
}
