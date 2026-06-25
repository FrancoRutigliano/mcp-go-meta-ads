package mcp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestMessageForError_SpanishAndNoLeak(t *testing.T) {
	cause := errors.New("graph error: http=400 code=190 token=super-secret")
	cases := []struct {
		kind     domain.Kind
		contains string
	}{
		{domain.KindUnauthorized, "credencial"},
		{domain.KindRateLimited, "Esperá"},
		{domain.KindNotFound, "No encontré"},
		{domain.KindInvalidInput, "no son válidos"},
		{domain.KindUpstream, "problema al consultar"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			err := domain.NewError(tc.kind, "op", cause)
			msg := messageForError(err)
			if !strings.Contains(msg, tc.contains) {
				t.Errorf("msg %q should contain %q", msg, tc.contains)
			}
			if strings.Contains(msg, "super-secret") || strings.Contains(msg, "code=190") {
				t.Errorf("msg leaked technical detail: %q", msg)
			}
		})
	}
}

func TestFormatCampaigns_Empty(t *testing.T) {
	out := formatCampaigns(nil, false)
	if !strings.Contains(out, "No hay campañas") {
		t.Errorf("unexpected empty message: %q", out)
	}
}

func TestFormatCampaigns_ListsAndTranslates(t *testing.T) {
	out := formatCampaigns([]domain.Campaign{
		{ID: "1", Name: "Ventas Q2", Status: domain.CampaignActive, Objective: "OUTCOME_SALES"},
	}, true)

	if !strings.Contains(out, "Ventas Q2") {
		t.Errorf("missing campaign name: %q", out)
	}
	if !strings.Contains(out, "activa") || !strings.Contains(out, "Ventas") {
		t.Errorf("status/objective not translated: %q", out)
	}
	if !strings.Contains(out, "Hay más campañas") {
		t.Errorf("truncation note missing: %q", out)
	}
}

func TestFormatInsights_EmptyShowsPeriod(t *testing.T) {
	applied := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	out := formatInsights(nil, applied)
	if !strings.Contains(out, "No hubo actividad") || !strings.Contains(out, "01/05/2026") {
		t.Errorf("empty insights message wrong: %q", out)
	}
}

func roas(v float64) *float64 { return &v }
func pur(v int64) *int64      { return &v }

func TestFormatInsights_RendersConversionMetrics(t *testing.T) {
	applied := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	out := formatInsights([]app.CampaignInsight{
		{
			Insight: domain.Insight{CampaignID: "1", CampaignName: "Ventas Q2", Range: applied, Metrics: domain.Metrics{
				Spend: 12345.67, Impressions: 100000, Reach: 80000,
				LinkCTR: 1.2, Frequency: 2.0, ROAS: roas(3.0), Purchases: pur(20),
			}},
			Eval: app.Evaluation{ROAS: domain.StatusOK, LinkCTR: domain.StatusOK, Frequency: domain.StatusOK},
		},
	}, applied)

	for _, want := range []string{"Ventas Q2", "12345.67", "ROAS", "3.00x", "✅", "compras: 20"} {
		if !strings.Contains(out, want) {
			t.Errorf("insights output missing %q in: %q", want, out)
		}
	}
}

// Principio IX: sin conversiones, ROAS/CPA se muestran como "no calculable", no 0.
func TestFormatInsights_NoConversionsNotZero(t *testing.T) {
	applied := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	out := formatInsights([]app.CampaignInsight{
		{
			Insight: domain.Insight{CampaignID: "2", CampaignName: "Mensajes", Metrics: domain.Metrics{Spend: 1000}},
			Eval:    app.Evaluation{ROAS: domain.StatusNoData, CPA: domain.StatusNoData, Insufficient: true},
		},
	}, applied)

	if !strings.Contains(out, "no calculable") {
		t.Errorf("debe decir 'no calculable' sin conversiones: %q", out)
	}
	if strings.Contains(out, "ROAS: ✅ 0.00x") || strings.Contains(out, "ROAS: ❌ 0.00x") {
		t.Errorf("no debe mostrar ROAS 0 como resultado: %q", out)
	}
	if !strings.Contains(out, "Datos insuficientes") {
		t.Errorf("debe advertir datos insuficientes: %q", out)
	}
}

func TestFormatBreakdown_RendersSegments(t *testing.T) {
	applied := domain.DateRange{
		Since: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
	}
	out := formatBreakdown(app.EvaluatedBreakdown{
		Dimension: domain.DimensionAge,
		Segments: []app.EvaluatedSegment{
			{Label: "25-34", Metrics: domain.Metrics{Spend: 1000, ROAS: roas(4.0)}, Eval: app.Evaluation{ROAS: domain.StatusOK}},
		},
	}, applied)

	for _, want := range []string{"edad", "25-34", "4.00x"} {
		if !strings.Contains(out, want) {
			t.Errorf("breakdown output missing %q in: %q", want, out)
		}
	}
}
