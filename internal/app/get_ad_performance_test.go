package app

import (
	"context"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestGetAdPerformance_SortsWinnersFirstAndAppliesDefaultLimit(t *testing.T) {
	fake := &fakeReader{ads: []domain.AdInsight{
		{AdID: "a", AdName: "flojo", Metrics: domain.Metrics{Impressions: 50000, LinkClicks: 1000, Purchases: ptrI(5), ROAS: ptrF(1.5)}},
		{AdID: "b", AdName: "sin roas", Metrics: domain.Metrics{Spend: 999}},
		{AdID: "c", AdName: "ganador", Metrics: domain.Metrics{Impressions: 50000, LinkClicks: 1000, Purchases: ptrI(9), ROAS: ptrF(6.0)}},
	}}
	uc := NewGetAdPerformance(fake, defThresholds(), defSufficiency())

	out, _, err := uc.Execute(context.Background(), "camp", &domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].Ad.AdName != "ganador" || out[1].Ad.AdName != "flojo" {
		t.Errorf("orden por ROAS incorrecto: %s, %s", out[0].Ad.AdName, out[1].Ad.AdName)
	}
	if out[2].Ad.AdName != "sin roas" {
		t.Errorf("el anuncio sin ROAS debe ir último, got %s", out[2].Ad.AdName)
	}
	if fake.gotAdQ.Limit != defaultAdLimit {
		t.Errorf("Limit = %d, want default %d", fake.gotAdQ.Limit, defaultAdLimit)
	}
	if out[0].Eval.ROAS != domain.StatusOK {
		t.Errorf("ganador con ROAS 6x debe ser cumple")
	}
}

func TestGetAdPerformance_InvalidRange(t *testing.T) {
	fake := &fakeReader{}
	uc := NewGetAdPerformance(fake, defThresholds(), defSufficiency())
	_, _, err := uc.Execute(context.Background(), "", &domain.DateRange{
		Since: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}, 10)
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("expected invalid_input, got %v", err)
	}
}
