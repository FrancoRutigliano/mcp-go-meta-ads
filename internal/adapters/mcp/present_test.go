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

func TestFormatFunnel_ShowsLeakAndHint(t *testing.T) {
	applied := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	out := formatFunnel([]app.CampaignFunnel{
		{
			CampaignID: "1", CampaignName: "Ventas", AnyActivity: true, HasLeak: true,
			LeakFrom: "Vistas de página", LeakTo: "Agregar al carrito", LeakPct: 93.3,
			Steps: []app.FunnelStep{
				{Name: "Impresiones", Count: 56648, Known: true},
				{Name: "Clics de enlace", Count: 2016, Known: true},
				{Name: "Vistas de página", Count: 1800, Known: true},
				{Name: "Agregar al carrito", Count: 120, Known: true},
			},
		},
	}, applied)

	for _, want := range []string{"Embudo", "Ventas", "Mayor caída", "93.3%", "no agregan al carrito"} {
		if !strings.Contains(out, want) {
			t.Errorf("funnel output missing %q in:\n%s", want, out)
		}
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

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name  string
		pesos float64
		want  string
	}{
		{name: "miles con separador local", pesos: 5000, want: "$5.000"},
		{name: "millones", pesos: 1250000, want: "$1.250.000"},
		{name: "menor a mil", pesos: 750, want: "$750"},
		{name: "cero", pesos: 0, want: "$0"},
		{name: "redondea a peso entero", pesos: 1999.6, want: "$2.000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMoney(domain.MoneyFromPesos(tt.pesos))
			if got != tt.want {
				t.Errorf("formatMoney(%v) = %q, esperaba %q", tt.pesos, got, tt.want)
			}
		})
	}
}

func TestMessageForError_GuardrailFactor(t *testing.T) {
	g := domain.Guardrails{MaxIncreaseFactor: 3, MaxDailyBudget: domain.MoneyFromPesos(25000)}
	err := g.Check(domain.MoneyFromPesos(1000), domain.MoneyFromPesos(9000), domain.BudgetDaily)

	msg := messageForError(err)

	// Debe decir cuál es el máximo real, no un mensaje genérico de "datos inválidos".
	if !strings.Contains(msg, "$3.000") {
		t.Errorf("el mensaje debería incluir el máximo admitido ($3.000), vino: %q", msg)
	}
	if strings.Contains(msg, "GuardrailError") || strings.Contains(msg, "domain.") {
		t.Errorf("el mensaje no debe filtrar detalle técnico, vino: %q", msg)
	}
}

func TestMessageForError_GuardrailCeiling(t *testing.T) {
	g := domain.Guardrails{MaxIncreaseFactor: 10, MaxDailyBudget: domain.MoneyFromPesos(25000)}
	err := g.Check(domain.MoneyFromPesos(20000), domain.MoneyFromPesos(30000), domain.BudgetDaily)

	msg := messageForError(err)

	if !strings.Contains(msg, "$25.000") {
		t.Errorf("el mensaje debería incluir el techo diario ($25.000), vino: %q", msg)
	}
}

func TestMessageForError_BudgetSentinels(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "nivel equivocado",
			err:      domain.NewError(domain.KindInvalidInput, "op", domain.ErrBudgetLevelMismatch),
			contains: "conjunto",
		},
		{
			name:     "sin base para el porcentaje",
			err:      domain.NewError(domain.KindInvalidInput, "op", domain.ErrNoBudgetToScale),
			contains: "porcentaje",
		},
		{
			name:     "monto y porcentaje a la vez",
			err:      domain.NewError(domain.KindInvalidInput, "op", domain.ErrBothAmountAndPercent),
			contains: "una sola",
		},
		{
			name:     "sin cambio",
			err:      domain.NewError(domain.KindInvalidInput, "op", domain.ErrBudgetUnchanged),
			contains: "ya tiene",
		},
		{
			name:     "base cambiada",
			err:      domain.NewError(domain.KindInvalidInput, "op", domain.ErrBudgetDrifted),
			contains: "cambió",
		},
		{
			name:     "sin conjuntos",
			err:      domain.NewError(domain.KindNotFound, "op", domain.ErrNoAdSets),
			contains: "conjuntos de anuncios",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := messageForError(tt.err)
			if !strings.Contains(strings.ToLower(msg), strings.ToLower(tt.contains)) {
				t.Errorf("mensaje = %q, esperaba que contuviera %q", msg, tt.contains)
			}
		})
	}
}

func TestMessageForError_FallsBackToKind(t *testing.T) {
	// Un error sin sentinela conocida sigue cayendo al switch por Kind.
	err := domain.NewError(domain.KindRateLimited, "op", nil)

	if msg := messageForError(err); !strings.Contains(msg, "limitando") {
		t.Errorf("mensaje = %q, esperaba el mensaje de rate limiting", msg)
	}
}
