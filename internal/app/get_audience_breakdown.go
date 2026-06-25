package app

import (
	"context"
	"fmt"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// EvaluatedSegment es un segmento de audiencia con su evaluación.
type EvaluatedSegment struct {
	Label   string
	Metrics domain.Metrics
	Eval    Evaluation
}

// EvaluatedBreakdown es el desglose por dimensión, ya evaluado.
type EvaluatedBreakdown struct {
	Dimension domain.BreakdownDimension
	Segments  []EvaluatedSegment
}

// GetAudienceBreakdown es el caso de uso para el desglose por audiencia.
type GetAudienceBreakdown struct {
	reader ports.MetaReader
	now    func() time.Time
	th     domain.Thresholds
	su     domain.SufficiencyPolicy
}

// NewGetAudienceBreakdown construye el caso de uso con reloj real.
func NewGetAudienceBreakdown(reader ports.MetaReader, th domain.Thresholds, su domain.SufficiencyPolicy) *GetAudienceBreakdown {
	return NewGetAudienceBreakdownWithClock(reader, th, su, time.Now)
}

// NewGetAudienceBreakdownWithClock permite inyectar un reloj (tests).
func NewGetAudienceBreakdownWithClock(reader ports.MetaReader, th domain.Thresholds, su domain.SufficiencyPolicy, now func() time.Time) *GetAudienceBreakdown {
	return &GetAudienceBreakdown{reader: reader, now: now, th: th, su: su}
}

// Execute devuelve el desglose evaluado por la dimensión pedida. campaignID
// vacío opera a nivel de cuenta. rng nil aplica los últimos 30 días.
func (uc *GetAudienceBreakdown) Execute(ctx context.Context, campaignID string, dim domain.BreakdownDimension, rng *domain.DateRange) (EvaluatedBreakdown, domain.DateRange, error) {
	const op = "app.GetAudienceBreakdown"

	if !dim.Valid() {
		return EvaluatedBreakdown{}, domain.DateRange{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("dimensión no soportada: %q", dim))
	}

	applied := uc.resolveRange(rng)
	if err := applied.Valid(); err != nil {
		return EvaluatedBreakdown{}, domain.DateRange{}, domain.NewError(domain.KindInvalidInput, op, err)
	}

	br, err := uc.reader.GetAudienceBreakdown(ctx, domain.AudienceQuery{
		CampaignID: campaignID,
		Dimension:  dim,
		Range:      applied,
	})
	if err != nil {
		return EvaluatedBreakdown{}, domain.DateRange{}, err
	}

	days := applied.Days()
	segments := make([]EvaluatedSegment, 0, len(br.Segments))
	for _, s := range br.Segments {
		segments = append(segments, EvaluatedSegment{
			Label:   s.Label,
			Metrics: s.Metrics,
			Eval:    Evaluate(s.Metrics, days, uc.th, uc.su),
		})
	}
	return EvaluatedBreakdown{Dimension: br.Dimension, Segments: segments}, applied, nil
}

func (uc *GetAudienceBreakdown) resolveRange(rng *domain.DateRange) domain.DateRange {
	if rng != nil {
		return *rng
	}
	until := truncateToDay(uc.now().UTC())
	since := until.AddDate(0, 0, -defaultInsightDays)
	return domain.DateRange{Since: since, Until: until}
}
