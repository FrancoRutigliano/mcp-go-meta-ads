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

// GetInsights es el caso de uso para leer rendimiento de campañas.
type GetInsights struct {
	reader ports.MetaReader
	now    func() time.Time // reloj inyectable para tests
}

// NewGetInsights construye el caso de uso con reloj real.
func NewGetInsights(reader ports.MetaReader) *GetInsights {
	return NewGetInsightsWithClock(reader, time.Now)
}

// NewGetInsightsWithClock permite inyectar un reloj (tests deterministas).
func NewGetInsightsWithClock(reader ports.MetaReader, now func() time.Time) *GetInsights {
	return &GetInsights{reader: reader, now: now}
}

// Execute devuelve el rendimiento por campaña. campaignID vacío consulta todas
// las campañas de la cuenta. Si rng es nil se aplica la ventana por defecto.
// Devuelve también el rango efectivamente aplicado, para que la presentación lo
// muestre de forma explícita (FR-004).
func (uc *GetInsights) Execute(ctx context.Context, campaignID string, rng *domain.DateRange) ([]domain.Insight, domain.DateRange, error) {
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
	return insights, applied, nil
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
