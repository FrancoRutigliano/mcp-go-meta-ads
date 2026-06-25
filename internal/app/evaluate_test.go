package app

import (
	"testing"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func ptrI(v int64) *int64     { return &v }
func ptrF(v float64) *float64 { return &v }

func baseThresholds() domain.Thresholds         { return domain.DefaultThresholds() }
func baseSufficiency() domain.SufficiencyPolicy { return domain.DefaultSufficiency() }

func TestEvaluate_GoodCampaign(t *testing.T) {
	m := domain.Metrics{
		Impressions: 50000, LinkClicks: 1000, LinkCTR: 1.2, Frequency: 2.0,
		Purchases: ptrI(20), Revenue: ptrF(1_600_000), ROAS: ptrF(3.0), CPA: ptrF(40_000),
	}
	e := Evaluate(m, 30, baseThresholds(), baseSufficiency())

	if e.ROAS != domain.StatusOK {
		t.Errorf("ROAS status = %q, want cumple", e.ROAS)
	}
	if e.LinkCTR != domain.StatusOK {
		t.Errorf("LinkCTR status = %q, want cumple", e.LinkCTR)
	}
	if e.Frequency != domain.StatusOK {
		t.Errorf("Frequency status = %q, want cumple", e.Frequency)
	}
	if e.CPA != domain.StatusOK { // ticket derivado = 1.6M/20 = 80k; CPA 40k <= 80k
		t.Errorf("CPA status = %q, want cumple", e.CPA)
	}
	if e.Insufficient {
		t.Errorf("no debería marcar insuficiente con 20 compras y 30 días")
	}
}

func TestEvaluate_LowROAS(t *testing.T) {
	m := domain.Metrics{
		Impressions: 50000, LinkClicks: 1000, LinkCTR: 1.0,
		Purchases: ptrI(10), Revenue: ptrF(300_000), ROAS: ptrF(1.5), CPA: ptrF(120_000),
	}
	e := Evaluate(m, 30, baseThresholds(), baseSufficiency())
	if e.ROAS != domain.StatusBad {
		t.Errorf("ROAS 1.5x debe ser no_cumple, got %q", e.ROAS)
	}
	if e.CPA != domain.StatusBad { // ticket derivado 30k; CPA 120k > 30k
		t.Errorf("CPA por encima del ticket debe ser no_cumple, got %q", e.CPA)
	}
}

// Principio IX: sin compras, ROAS/CPA = sin_datos, e Insufficient = true.
func TestEvaluate_NoConversions_NoData(t *testing.T) {
	m := domain.Metrics{
		Impressions: 50000, LinkClicks: 1000, LinkCTR: 1.0,
		// Purchases/Revenue/ROAS/CPA nil
	}
	e := Evaluate(m, 30, baseThresholds(), baseSufficiency())
	if e.ROAS != domain.StatusNoData || e.CPA != domain.StatusNoData {
		t.Errorf("sin compras ROAS/CPA deben ser sin_datos, got ROAS=%q CPA=%q", e.ROAS, e.CPA)
	}
	if !e.Insufficient {
		t.Errorf("sin compras debe marcar Insufficient")
	}
}

func TestEvaluate_FewDays_InsufficientForConversion(t *testing.T) {
	m := domain.Metrics{
		Impressions: 50000, LinkClicks: 1000, LinkCTR: 1.0,
		Purchases: ptrI(3), Revenue: ptrF(240_000), ROAS: ptrF(5.0), CPA: ptrF(10_000),
	}
	// Solo 2 días (< MinDays 7) → no concluir ROAS/CPA aunque haya compras.
	e := Evaluate(m, 2, baseThresholds(), baseSufficiency())
	if e.ROAS != domain.StatusNoData || e.CPA != domain.StatusNoData {
		t.Errorf("con pocos días ROAS/CPA deben ser sin_datos, got ROAS=%q CPA=%q", e.ROAS, e.CPA)
	}
	if !e.Insufficient {
		t.Errorf("pocos días debe marcar Insufficient")
	}
}

func TestEvaluate_LinkCTR_Bounds(t *testing.T) {
	suff := domain.Metrics{Impressions: 50000, LinkClicks: 1000}

	low := suff
	low.LinkCTR = 0.5
	if Evaluate(low, 30, baseThresholds(), baseSufficiency()).LinkCTR != domain.StatusWarn {
		t.Errorf("CTR 0.5%% (fuera de rango) debe ser atención")
	}

	ok := suff
	ok.LinkCTR = 1.0
	if Evaluate(ok, 30, baseThresholds(), baseSufficiency()).LinkCTR != domain.StatusOK {
		t.Errorf("CTR 1.0%% debe ser cumple")
	}
}

func TestEvaluate_LinkCTR_InsufficientSample(t *testing.T) {
	m := domain.Metrics{Impressions: 100, LinkClicks: 5, LinkCTR: 1.0} // muestra ínfima
	if Evaluate(m, 30, baseThresholds(), baseSufficiency()).LinkCTR != domain.StatusNoData {
		t.Errorf("muestra ínfima debe dar LinkCTR sin_datos")
	}
}

func TestEvaluate_FrequencyFatigue(t *testing.T) {
	m := domain.Metrics{Impressions: 50000, LinkClicks: 1000, Frequency: 4.2}
	if Evaluate(m, 30, baseThresholds(), baseSufficiency()).Frequency != domain.StatusWarn {
		t.Errorf("frecuencia 4.2 (>3.5) debe alertar (atención)")
	}
}

func TestEvaluate_TicketDefaultWhenNoRevenue(t *testing.T) {
	// Compras suficientes pero sin Revenue → usa ticket default 80k.
	m := domain.Metrics{
		Impressions: 50000, LinkClicks: 1000,
		Purchases: ptrI(10), ROAS: ptrF(3.0), CPA: ptrF(50_000),
	}
	e := Evaluate(m, 30, baseThresholds(), baseSufficiency())
	if e.TicketUsed != 80000 {
		t.Errorf("ticket usado = %v, want default 80000", e.TicketUsed)
	}
	if e.CPA != domain.StatusOK { // 50k <= 80k
		t.Errorf("CPA 50k vs ticket 80k debe ser cumple, got %q", e.CPA)
	}
}
