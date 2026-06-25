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

// Days devuelve la cantidad de días del período, inclusivo en ambos extremos
// (un rango del mismo día cuenta 1). Se usa para la política de suficiencia.
func (r DateRange) Days() int {
	if r.Since.IsZero() || r.Until.IsZero() {
		return 0
	}
	return int(r.Until.Sub(r.Since).Hours()/24) + 1
}

// Metrics son las métricas de rendimiento que exponen las tools de lectura.
//
// Las métricas de conversión que dependen del píxel (ROAS, CPA, compras,
// facturación) son PUNTEROS: nil significa "no calculable / sin datos", que es
// semánticamente distinto de 0 (Principio IX). La capa de presentación nunca
// debe mostrar 0 cuando el valor es nil.
type Metrics struct {
	// Básicas (feature 001).
	Spend       float64 // gasto en la moneda de la cuenta
	Impressions int64
	Clicks      int64 // todos los clics
	Reach       int64
	CTR         float64 // click-through rate de todos los clics (%)
	CPC         float64 // costo por clic

	// Enlace (para evaluar el umbral 0,8–1,5% correctamente).
	LinkClicks int64
	LinkCTR    float64 // CTR de enlace (%)

	// Entrega.
	Frequency float64 // frecuencia directa de Meta

	// Conversión (nullable: nil = no calculable).
	Purchases *int64   // cantidad de compras atribuidas
	Revenue   *float64 // facturación atribuida (valor de las compras)
	ROAS      *float64 // retorno sobre inversión publicitaria (directo de Meta)
	CPA       *float64 // costo por compra
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
