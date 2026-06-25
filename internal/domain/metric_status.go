package domain

// MetricStatus es el estado de una métrica frente a su umbral de negocio
// (Principios VIII y IX). La capa de presentación lo traduce a íconos/español.
type MetricStatus string

const (
	StatusOK     MetricStatus = "cumple"    // dentro del umbral deseado
	StatusWarn   MetricStatus = "atencion"  // fuera de rango, requiere revisión
	StatusBad    MetricStatus = "no_cumple" // por debajo del umbral (candidata a apagar)
	StatusNoData MetricStatus = "sin_datos" // muestra insuficiente / no calculable
)

// Thresholds reúne los umbrales de negocio (Principio VIII). Todos ajustables
// por configuración; estos son los defaults.
type Thresholds struct {
	MinROAS      float64 // ROAS mínimo aceptable (>=)
	LinkCTRMin   float64 // CTR de enlace mínimo, en %
	LinkCTRMax   float64 // CTR de enlace máximo, en %
	MaxFrequency float64 // frecuencia máxima antes de alertar fatiga
	AvgTicket    float64 // ticket promedio por compra (ARS), para comparar el CPA
}

// DefaultThresholds devuelve los valores por defecto de la constitución y del
// negocio (ticket real de Tienda Nube ≈ 80.000 ARS).
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinROAS:      2.0,
		LinkCTRMin:   0.8,
		LinkCTRMax:   1.5,
		MaxFrequency: 3.5,
		AvgTicket:    80000,
	}
}

// SufficiencyPolicy define cuándo una muestra es demasiado chica para concluir
// (Principio IX). Por debajo de estos mínimos no se emiten recomendaciones.
type SufficiencyPolicy struct {
	MinPurchases   int64 // compras mínimas para calcular ROAS/CPA
	MinDays        int   // días mínimos de período para calcular ROAS/CPA
	MinImpressions int64 // impresiones mínimas para evaluar métricas de tasa
	MinLinkClicks  int64 // clics de enlace mínimos para evaluar CTR de enlace
}

// DefaultSufficiency devuelve los mínimos conservadores por defecto.
func DefaultSufficiency() SufficiencyPolicy {
	return SufficiencyPolicy{
		MinPurchases:   1,
		MinDays:        7,
		MinImpressions: 1000,
		MinLinkClicks:  50,
	}
}
