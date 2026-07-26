package meta

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestParseInsights_ConversionFields(t *testing.T) {
	body := []byte(`{"data":[{
		"campaign_id":"1","campaign_name":"Ventas",
		"spend":"312760.38","impressions":"63185","clicks":"2269","reach":"30773",
		"ctr":"3.59","cpc":"137.84","frequency":"2.05",
		"inline_link_clicks":"1800","inline_link_click_ctr":"1.10",
		"purchase_roas":[{"action_type":"omni_purchase","value":"2.34"}],
		"actions":[{"action_type":"landing_page_view","value":"500"},{"action_type":"purchase","value":"12"}],
		"action_values":[{"action_type":"purchase","value":"731860.00"}],
		"cost_per_action_type":[{"action_type":"purchase","value":"26063.36"}],
		"date_start":"2026-05-25","date_stop":"2026-06-24"
	}]}`)

	out, err := parseInsights(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := out[0].Metrics
	if m.Purchases == nil || *m.Purchases != 12 {
		t.Errorf("Purchases = %v, want 12", m.Purchases)
	}
	if m.Revenue == nil || *m.Revenue != 731860.00 {
		t.Errorf("Revenue = %v, want 731860", m.Revenue)
	}
	if m.ROAS == nil || *m.ROAS != 2.34 {
		t.Errorf("ROAS = %v, want 2.34", m.ROAS)
	}
	if m.CPA == nil || *m.CPA != 26063.36 {
		t.Errorf("CPA = %v, want 26063.36", m.CPA)
	}
	if m.LinkClicks != 1800 || m.LinkCTR != 1.10 || m.Frequency != 2.05 {
		t.Errorf("link/frequency mal parseados: linkClicks=%d linkCTR=%v freq=%v", m.LinkClicks, m.LinkCTR, m.Frequency)
	}
}

// Principio IX: sin compras, los punteros de conversión deben quedar nil (no 0).
func TestParseInsights_NoConversions_NilPointers(t *testing.T) {
	body := []byte(`{"data":[{
		"campaign_id":"2","campaign_name":"Mensajes",
		"spend":"1000","impressions":"5000","clicks":"80","reach":"4000",
		"actions":[{"action_type":"link_click","value":"80"}],
		"date_start":"2026-06-01","date_stop":"2026-06-24"
	}]}`)

	out, err := parseInsights(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := out[0].Metrics
	if m.Purchases != nil || m.Revenue != nil || m.ROAS != nil || m.CPA != nil {
		t.Errorf("sin compras los punteros deben ser nil: %+v", m)
	}
}

func TestFindAction_PrefersPurchaseOverOmni(t *testing.T) {
	list := []actionValue{
		{ActionType: "omni_purchase", Value: "99"},
		{ActionType: "purchase", Value: "12"},
	}
	v, ok := findAction(list, purchaseActionTypes)
	if !ok || v != 12 {
		t.Errorf("findAction priorizó mal: got %v ok=%v, want 12", v, ok)
	}
}

func TestMetricsFrom_ParsesFunnelSteps(t *testing.T) {
	body := []byte(`{"data":[{
		"campaign_id":"1","impressions":"10000","inline_link_clicks":"500",
		"actions":[
			{"action_type":"landing_page_view","value":"450"},
			{"action_type":"add_to_cart","value":"120"},
			{"action_type":"initiate_checkout","value":"40"},
			{"action_type":"purchase","value":"12"}
		]
	}]}`)
	out, err := parseInsights(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := out[0].Metrics
	if m.LandingPageViews == nil || *m.LandingPageViews != 450 {
		t.Errorf("LandingPageViews = %v, want 450", m.LandingPageViews)
	}
	if m.AddToCart == nil || *m.AddToCart != 120 {
		t.Errorf("AddToCart = %v, want 120", m.AddToCart)
	}
	if m.InitiateCheckout == nil || *m.InitiateCheckout != 40 {
		t.Errorf("InitiateCheckout = %v, want 40", m.InitiateCheckout)
	}
}

func TestMetricsFrom_FunnelStepsNilWhenPixelSilent(t *testing.T) {
	body := []byte(`{"data":[{"campaign_id":"1","impressions":"10000","inline_link_clicks":"500",
		"actions":[{"action_type":"link_click","value":"500"}]}]}`)
	out, _ := parseInsights(body)
	m := out[0].Metrics
	if m.AddToCart != nil || m.InitiateCheckout != nil || m.LandingPageViews != nil {
		t.Errorf("pasos no reportados por el pixel deben quedar nil: %+v", m)
	}
}

func TestGetAdInsights_HappyPath(t *testing.T) {
	var gotPath, gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"ad_id":"111","ad_name":"Piluso Rafia","campaign_name":"Ventas","spend":"5000",
			 "purchase_roas":[{"action_type":"omni_purchase","value":"4.2"}]}
		]}`))
	})

	rng := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	out, err := client.GetAdInsights(context.Background(), domain.AdQuery{CampaignID: "6960821349863", Range: rng, Limit: 25})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].AdName != "Piluso Rafia" {
		t.Fatalf("ad insights inesperados: %+v", out)
	}
	if out[0].Metrics.ROAS == nil || *out[0].Metrics.ROAS != 4.2 {
		t.Errorf("ROAS del anuncio mal mapeado")
	}
	if !strings.Contains(gotPath, "/6960821349863/insights") {
		t.Errorf("path = %q, want campaign insights", gotPath)
	}
	if !strings.Contains(gotQuery, "level=ad") {
		t.Errorf("query debe incluir level=ad: %q", gotQuery)
	}
}

func TestGetAudienceBreakdown_HappyPath(t *testing.T) {
	var gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"age":"25-34","spend":"1000","impressions":"20000","inline_link_clicks":"300","inline_link_click_ctr":"1.5",
			 "actions":[{"action_type":"purchase","value":"5"}],"action_values":[{"action_type":"purchase","value":"400000"}],
			 "purchase_roas":[{"action_type":"omni_purchase","value":"4.0"}]},
			{"age":"35-44","spend":"800","impressions":"15000","inline_link_clicks":"100","inline_link_click_ctr":"0.6"}
		]}`))
	})

	rng := domain.DateRange{
		Since: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
	}
	br, err := client.GetAudienceBreakdown(context.Background(), domain.AudienceQuery{
		Dimension: domain.DimensionAge,
		Range:     rng,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "breakdowns=age") {
		t.Errorf("query debe incluir breakdowns=age: %q", gotQuery)
	}
	if len(br.Segments) != 2 || br.Segments[0].Label != "25-34" {
		t.Fatalf("segmentos inesperados: %+v", br.Segments)
	}
	if br.Segments[0].Metrics.ROAS == nil || *br.Segments[0].Metrics.ROAS != 4.0 {
		t.Errorf("ROAS del segmento 0 mal mapeado")
	}
}

func TestGetAudienceBreakdown_InvalidDimension_NoHTTP(t *testing.T) {
	var hits int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"data":[]}`))
	})

	rng := domain.DateRange{
		Since: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
	}
	_, err := client.GetAudienceBreakdown(context.Background(), domain.AudienceQuery{
		Dimension: "country", // no soportada
		Range:     rng,
	})
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("expected invalid_input, got %v", err)
	}
	if hits != 0 {
		t.Errorf("no debe llamar a Meta con dimensión inválida (hits=%d)", hits)
	}
}

func TestGetAudienceBreakdown_InvalidComboMapsToInvalidInput(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"(#100) breakdowns[0] must be a valid insights breakdown","type":"OAuthException","code":100}}`))
	})

	rng := domain.DateRange{
		Since: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
	}
	_, err := client.GetAudienceBreakdown(context.Background(), domain.AudienceQuery{
		Dimension: domain.DimensionRegion,
		Range:     rng,
	})
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("expected invalid_input from breakdown error, got kind=%q err=%v", domain.KindOf(err), err)
	}
}
