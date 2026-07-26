package app

import (
	"context"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestGetFunnel_DetectsBiggestLeak(t *testing.T) {
	fake := &fakeReader{insights: []domain.Insight{
		{CampaignID: "1", CampaignName: "Ventas", Metrics: domain.Metrics{
			Impressions:      56648,
			LinkClicks:       2016,
			LandingPageViews: ptrI(1800),
			AddToCart:        ptrI(120), // caída fuerte acá: 1800 → 120
			InitiateCheckout: ptrI(45),
			Purchases:        ptrI(22),
		}},
	}}
	uc := NewGetFunnel(fake)

	out, _, err := uc.Execute(context.Background(), "1", &domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := out[0]
	if !f.HasLeak {
		t.Fatal("debería detectar una caída")
	}
	// 1800 → 120 es la mayor caída (~93%).
	if f.LeakFrom != "Vistas de página" || f.LeakTo != "Agregar al carrito" {
		t.Errorf("mayor caída mal detectada: de %q a %q", f.LeakFrom, f.LeakTo)
	}
	if f.LeakPct < 90 {
		t.Errorf("LeakPct = %.1f, want ~93", f.LeakPct)
	}
	if f.PixelGaps {
		t.Errorf("no debería marcar PixelGaps con todos los pasos presentes")
	}
}

func TestGetFunnel_PixelGapsWhenStepsMissing(t *testing.T) {
	fake := &fakeReader{insights: []domain.Insight{
		{CampaignID: "2", CampaignName: "Mensajes", Metrics: domain.Metrics{
			Impressions: 10000,
			LinkClicks:  500,
			Purchases:   ptrI(5),
			// sin LandingPageViews/AddToCart/InitiateCheckout
		}},
	}}
	uc := NewGetFunnel(fake)

	out, _, err := uc.Execute(context.Background(), "2", &domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out[0].PixelGaps {
		t.Errorf("debería marcar PixelGaps cuando faltan pasos intermedios")
	}
}

func TestGetFunnel_InvalidRange(t *testing.T) {
	fake := &fakeReader{}
	uc := NewGetFunnel(fake)
	_, _, err := uc.Execute(context.Background(), "", &domain.DateRange{
		Since: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("expected invalid_input, got %v", err)
	}
}
