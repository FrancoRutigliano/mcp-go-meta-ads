package app

import (
	"context"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// defaultInsightDays es la ventana por defecto cuando el usuario no indica un
// período (FR-004): los últimos 30 días.
const defaultInsightDays = 30

// CampaignInsight combina el rendimiento crudo de una campaña con su evaluación
// contra los umbrales de negocio (Principios VIII/IX).
type CampaignInsight struct {
	Insight domain.Insight
	Eval    Evaluation
}

// GetInsights es el caso de uso para leer y evaluar rendimiento de campañas.
type GetInsights struct {
	reader ports.MetaReader
	now    func() time.Time // reloj inyectable para tests
	th     domain.Thresholds
	su     domain.SufficiencyPolicy
}

// NewGetInsights construye el caso de uso con reloj real.
func NewGetInsights(reader ports.MetaReader, th domain.Thresholds, su domain.SufficiencyPolicy) *GetInsights {
	return NewGetInsightsWithClock(reader, th, su, time.Now)
}

// NewGetInsightsWithClock permite inyectar un reloj (tests deterministas).
func NewGetInsightsWithClock(reader ports.MetaReader, th domain.Thresholds, su domain.SufficiencyPolicy, now func() time.Time) *GetInsights {
	return &GetInsights{reader: reader, now: now, th: th, su: su}
}

// Execute devuelve el rendimiento evaluado por campaña. campaignID vacío
// consulta todas las campañas de la cuenta. Si rng es nil se aplica la ventana
// por defecto. Devuelve también el rango aplicado para mostrarlo (FR-004).
func (uc *GetInsights) Execute(ctx context.Context, campaignID string, rng *domain.DateRange) ([]CampaignInsight, domain.DateRange, error) {
	applied := uc.resolveRange(rng)

	if err := applied.Valid(); err != nil {
		return nil, domain.DateRange{}, domain.NewError(domain.KindInvalidInput, "app.GetInsights", err)
	}

	insights, err := uc.reader.GetInsights(ctx, domain.InsightQuery{
		CampaignID: campaignID,
		Range:      applied,
	})
	if err != nil {
		return nil, domain.DateRange{}, err
	}

	days := applied.Days()
	out := make([]CampaignInsight, 0, len(insights))
	for _, in := range insights {
		out = append(out, CampaignInsight{
			Insight: in,
			Eval:    Evaluate(in.Metrics, days, uc.th, uc.su),
		})
	}
	return out, applied, nil
}

// resolveRange aplica el default de 30 días cuando no se especifica período,
// truncando a fecha (sin hora) porque Meta opera a nivel de día.
func (uc *GetInsights) resolveRange(rng *domain.DateRange) domain.DateRange {
	if rng != nil {
		return *rng
	}
	until := truncateToDay(uc.now().UTC())
	since := until.AddDate(0, 0, -defaultInsightDays)
	return domain.DateRange{Since: since, Until: until}
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
