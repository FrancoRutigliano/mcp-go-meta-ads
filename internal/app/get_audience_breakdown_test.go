package app

import (
	"context"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestGetAudienceBreakdown_InvalidDimension(t *testing.T) {
	fake := &fakeReader{}
	uc := NewGetAudienceBreakdown(fake, defThresholds(), defSufficiency())

	_, _, err := uc.Execute(context.Background(), "", domain.BreakdownDimension("country"), nil)
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("expected invalid_input, got %v", err)
	}
	if fake.called {
		t.Errorf("no debe llamar al reader con dimensión inválida")
	}
}

func TestGetAudienceBreakdown_EvaluatesSegmentsAndDefaultRange(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	fake := &fakeReader{
		breakdown: domain.AudienceBreakdown{
			Dimension: domain.DimensionAge,
			Segments: []domain.AudienceSegment{
				{Label: "25-34", Metrics: domain.Metrics{
					Impressions: 50000, LinkClicks: 1000, LinkCTR: 1.2,
					Purchases: ptrI(20), Revenue: ptrF(1_600_000), ROAS: ptrF(3.0), CPA: ptrF(40_000),
				}},
			},
		},
	}
	uc := NewGetAudienceBreakdownWithClock(fake, defThresholds(), defSufficiency(), func() time.Time { return now })

	out, applied, err := uc.Execute(context.Background(), "123", domain.DimensionAge, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Dimension != domain.DimensionAge || len(out.Segments) != 1 {
		t.Fatalf("breakdown inesperado: %+v", out)
	}
	if out.Segments[0].Eval.ROAS != domain.StatusOK {
		t.Errorf("segmento con ROAS 3x debe ser cumple, got %q", out.Segments[0].Eval.ROAS)
	}
	// Default range = últimos 30 días respecto del reloj fijo.
	wantUntil := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	if !applied.Until.Equal(wantUntil) {
		t.Errorf("rango aplicado.Until = %v, want %v", applied.Until, wantUntil)
	}
	if fake.gotAudienceQ.CampaignID != "123" || fake.gotAudienceQ.Dimension != domain.DimensionAge {
		t.Errorf("query al reader inesperada: %+v", fake.gotAudienceQ)
	}
}

func TestGetAudienceBreakdown_PropagatesError(t *testing.T) {
	fake := &fakeReader{err: domain.NewError(domain.KindRateLimited, "meta", nil)}
	uc := NewGetAudienceBreakdown(fake, defThresholds(), defSufficiency())

	_, _, err := uc.Execute(context.Background(), "", domain.DimensionGender, nil)
	if domain.KindOf(err) != domain.KindRateLimited {
		t.Errorf("expected rate_limited propagated, got %v", err)
	}
}
