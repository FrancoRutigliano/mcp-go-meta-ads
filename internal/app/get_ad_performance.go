package app

import (
	"context"
	"sort"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// defaultAdLimit acota la respuesta cuando el llamador no fija un tope: una
// cuenta puede tener muchos anuncios.
const defaultAdLimit = 25

// AdReport combina el rendimiento de un anuncio con su evaluación vs umbrales.
type AdReport struct {
	Ad   domain.AdInsight
	Eval Evaluation
}

// GetAdPerformance es el caso de uso para leer y rankear anuncios.
type GetAdPerformance struct {
	reader ports.MetaReader
	now    func() time.Time
	th     domain.Thresholds
	su     domain.SufficiencyPolicy
}

// NewGetAdPerformance construye el caso de uso con reloj real.
func NewGetAdPerformance(reader ports.MetaReader, th domain.Thresholds, su domain.SufficiencyPolicy) *GetAdPerformance {
	return NewGetAdPerformanceWithClock(reader, th, su, time.Now)
}

// NewGetAdPerformanceWithClock permite inyectar un reloj (tests).
func NewGetAdPerformanceWithClock(reader ports.MetaReader, th domain.Thresholds, su domain.SufficiencyPolicy, now func() time.Time) *GetAdPerformance {
	return &GetAdPerformance{reader: reader, now: now, th: th, su: su}
}

// Execute devuelve los anuncios evaluados y ordenados de mejor a peor (por ROAS;
// los que no tienen ROAS calculable van al final, ordenados por gasto). limit<=0
// aplica el tope por defecto. campaignID vacío consulta toda la cuenta.
func (uc *GetAdPerformance) Execute(ctx context.Context, campaignID string, rng *domain.DateRange, limit int) ([]AdReport, domain.DateRange, error) {
	applied := resolveRangeWith(uc.now, rng)
	if err := applied.Valid(); err != nil {
		return nil, domain.DateRange{}, domain.NewError(domain.KindInvalidInput, "app.GetAdPerformance", err)
	}
	if limit <= 0 {
		limit = defaultAdLimit
	}

	ads, err := uc.reader.GetAdInsights(ctx, domain.AdQuery{
		CampaignID: campaignID,
		Range:      applied,
		Limit:      limit,
	})
	if err != nil {
		return nil, domain.DateRange{}, err
	}

	days := applied.Days()
	reports := make([]AdReport, 0, len(ads))
	for _, ad := range ads {
		reports = append(reports, AdReport{Ad: ad, Eval: Evaluate(ad.Metrics, days, uc.th, uc.su)})
	}
	sortByWinners(reports)
	return reports, applied, nil
}

// sortByWinners ordena los anuncios: primero los que tienen ROAS (de mayor a
// menor), y al final los que no lo tienen (por gasto descendente).
func sortByWinners(reports []AdReport) {
	sort.SliceStable(reports, func(i, j int) bool {
		a, b := reports[i].Ad.Metrics, reports[j].Ad.Metrics
		switch {
		case a.ROAS != nil && b.ROAS != nil:
			return *a.ROAS > *b.ROAS
		case a.ROAS != nil:
			return true // los que tienen ROAS van primero
		case b.ROAS != nil:
			return false
		default:
			return a.Spend > b.Spend
		}
	})
}
