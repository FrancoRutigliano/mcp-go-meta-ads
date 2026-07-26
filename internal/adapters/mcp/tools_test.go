package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/mashats/meta-ads-manager/internal/adapters/memstore"
	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

// noopWriter es un MetaWriter de prueba que registra la última escritura.
type noopWriter struct {
	calls  int
	status domain.CampaignStatus
	err    error
}

func (w *noopWriter) UpdateCampaignStatus(_ context.Context, _ string, s domain.CampaignStatus) error {
	w.calls++
	w.status = s
	return w.err
}

type fakeReader struct {
	campaigns []domain.Campaign
	campaign  domain.Campaign
	insights  []domain.Insight
	breakdown domain.AudienceBreakdown
	ads       []domain.AdInsight
	err       error
}

func (f *fakeReader) ListCampaigns(context.Context, domain.CampaignQuery) ([]domain.Campaign, error) {
	return f.campaigns, f.err
}
func (f *fakeReader) GetInsights(context.Context, domain.InsightQuery) ([]domain.Insight, error) {
	return f.insights, f.err
}
func (f *fakeReader) GetAudienceBreakdown(context.Context, domain.AudienceQuery) (domain.AudienceBreakdown, error) {
	return f.breakdown, f.err
}
func (f *fakeReader) GetAdInsights(context.Context, domain.AdQuery) ([]domain.AdInsight, error) {
	return f.ads, f.err
}
func (f *fakeReader) GetCampaign(context.Context, string) (domain.Campaign, error) {
	return f.campaign, f.err
}

func th() domain.Thresholds        { return domain.DefaultThresholds() }
func su() domain.SufficiencyPolicy { return domain.DefaultSufficiency() }

func newRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(r *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestCampaignsHandler_Success(t *testing.T) {
	fake := &fakeReader{campaigns: []domain.Campaign{
		{ID: "1", Name: "Ventas Q2", Status: domain.CampaignActive, Objective: "OUTCOME_SALES"},
	}}
	h := campaignsHandler(app.NewListCampaigns(fake))

	res, err := h(context.Background(), newRequest(map[string]any{"status": "active"}))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %q", resultText(res))
	}
	if !strings.Contains(resultText(res), "Ventas Q2") {
		t.Errorf("missing campaign in output: %q", resultText(res))
	}
}

func TestCampaignsHandler_UpstreamErrorYieldsSpanishMessage(t *testing.T) {
	fake := &fakeReader{err: domain.NewError(domain.KindUnauthorized, "meta.ListCampaigns", errors.New("code=190 secret-token"))}
	h := campaignsHandler(app.NewListCampaigns(fake))

	res, err := h(context.Background(), newRequest(nil))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	msg := resultText(res)
	if !strings.Contains(msg, "credencial") {
		t.Errorf("expected Spanish unauthorized message, got %q", msg)
	}
	if strings.Contains(msg, "secret-token") || strings.Contains(msg, "190") {
		t.Errorf("error message leaked technical detail: %q", msg)
	}
}

func TestInsightsHandler_DefaultPeriod(t *testing.T) {
	fake := &fakeReader{insights: []domain.Insight{
		{CampaignID: "1", CampaignName: "Ventas Q2", Metrics: domain.Metrics{Spend: 1000}},
	}}
	h := insightsHandler(app.NewGetInsights(fake, th(), su()))

	res, err := h(context.Background(), newRequest(nil))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got: %q", resultText(res))
	}
	if !strings.Contains(resultText(res), "Rendimiento") {
		t.Errorf("unexpected output: %q", resultText(res))
	}
}

func TestInsightsHandler_IncompleteRangeRejected(t *testing.T) {
	fake := &fakeReader{}
	h := insightsHandler(app.NewGetInsights(fake, th(), su()))

	// Sólo 'since' sin 'until' → debe rechazar con mensaje claro, sin llamar a Meta.
	res, err := h(context.Background(), newRequest(map[string]any{"since": "2026-05-01"}))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for incomplete range")
	}
	if !strings.Contains(resultText(res), "ambas fechas") {
		t.Errorf("unexpected message: %q", resultText(res))
	}
}

func TestAudienceHandler_Success(t *testing.T) {
	v := 4.0
	fake := &fakeReader{breakdown: domain.AudienceBreakdown{
		Dimension: domain.DimensionAge,
		Segments: []domain.AudienceSegment{
			{Label: "25-34", Metrics: domain.Metrics{Spend: 1000, ROAS: &v}},
		},
	}}
	h := audienceHandler(app.NewGetAudienceBreakdown(fake, th(), su()))

	res, err := h(context.Background(), newRequest(map[string]any{"dimension": "age"}))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got: %q", resultText(res))
	}
	out := resultText(res)
	if !strings.Contains(out, "edad") || !strings.Contains(out, "25-34") {
		t.Errorf("breakdown output inesperado: %q", out)
	}
}

func TestAudienceHandler_InvalidDimension(t *testing.T) {
	fake := &fakeReader{}
	h := audienceHandler(app.NewGetAudienceBreakdown(fake, th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{"dimension": "country"}))
	if !res.IsError {
		t.Fatal("expected error result for invalid dimension")
	}
}

func TestFunnelHandler_Success(t *testing.T) {
	fake := &fakeReader{insights: []domain.Insight{
		{CampaignID: "1", CampaignName: "Ventas", Metrics: domain.Metrics{
			Impressions: 10000, LinkClicks: 500,
			LandingPageViews: pur(400), AddToCart: pur(50), InitiateCheckout: pur(20), Purchases: pur(8),
		}},
	}}
	h := funnelHandler(app.NewGetFunnel(fake))

	res, err := h(context.Background(), newRequest(nil))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got: %q", resultText(res))
	}
	if !strings.Contains(resultText(res), "Embudo") {
		t.Errorf("unexpected funnel output: %q", resultText(res))
	}
}

func TestAdPerformanceHandler_Success(t *testing.T) {
	fake := &fakeReader{ads: []domain.AdInsight{
		{AdID: "a1", AdName: "Piluso Rafia", CampaignName: "Ventas", Metrics: domain.Metrics{Spend: 1000, ROAS: roas(5.0)}},
	}}
	h := adPerformanceHandler(app.NewGetAdPerformance(fake, th(), su()))

	res, err := h(context.Background(), newRequest(map[string]any{"campaign_id": "c1"}))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got: %q", resultText(res))
	}
	out := resultText(res)
	if !strings.Contains(out, "Piluso Rafia") || !strings.Contains(out, "Ventas") {
		t.Errorf("ad performance output inesperado: %q", out)
	}
}

func TestProposeThenConfirm_Flow(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{ID: "1", Name: "Ventas", Status: domain.CampaignActive}}
	store := memstore.New()
	writer := &noopWriter{}

	propose := proposeCampaignStatusHandler(app.NewProposeCampaignStatus(fake, store))
	confirm := confirmActionHandler(app.NewConfirmProposal(store, writer, nil))

	// 1) propose: no debe escribir, y devuelve un proposal_id.
	res, _ := propose(context.Background(), newRequest(map[string]any{"campaign_id": "1", "action": "pause"}))
	if res.IsError {
		t.Fatalf("propose falló: %q", resultText(res))
	}
	out := resultText(res)
	if writer.calls != 0 {
		t.Fatal("propose NO debe escribir en Meta")
	}
	// Extraer el proposal_id del texto (formato prop_...).
	idx := strings.Index(out, "prop_")
	if idx < 0 {
		t.Fatalf("no encontré proposal_id en: %q", out)
	}
	id := strings.Fields(out[idx:])[0]

	// 2) confirm: aplica el cambio.
	res2, _ := confirm(context.Background(), newRequest(map[string]any{"proposal_id": id, "confirmed_by": "Mariana"}))
	if res2.IsError {
		t.Fatalf("confirm falló: %q", resultText(res2))
	}
	if writer.calls != 1 || writer.status != domain.CampaignPaused {
		t.Errorf("confirm no aplicó el cambio: %+v", writer)
	}
	if !strings.Contains(resultText(res2), "Hecho") {
		t.Errorf("confirmación inesperada: %q", resultText(res2))
	}
}

// Principio II: confirmar sin un propose previo válido debe fallar.
func TestConfirm_WithoutProposeRejected(t *testing.T) {
	confirm := confirmActionHandler(app.NewConfirmProposal(memstore.New(), &noopWriter{}, nil))
	res, _ := confirm(context.Background(), newRequest(map[string]any{"proposal_id": "prop_inventado"}))
	if !res.IsError {
		t.Fatal("confirmar un id inexistente debe dar error")
	}
}

func TestToolBuilders_HaveExpectedNames(t *testing.T) {
	cases := map[string]string{
		"get_campaigns":           campaignsTool().Name,
		"get_campaigns_insights":  insightsTool().Name,
		"get_audience_breakdown":  audienceTool().Name,
		"get_conversion_funnel":   funnelTool().Name,
		"get_ad_performance":      adPerformanceTool().Name,
		"propose_campaign_status": proposeCampaignStatusTool().Name,
		"confirm_action":          confirmActionTool().Name,
	}
	for want, got := range cases {
		if got != want {
			t.Errorf("tool name = %q, want %q", got, want)
		}
	}
}

func TestNewServer_BuildsWithAllUseCases(t *testing.T) {
	fake := &fakeReader{}
	store := memstore.New()
	srv := NewServer("test", "0.0.0", Deps{
		ListCampaigns: app.NewListCampaigns(fake),
		Insights:      app.NewGetInsights(fake, th(), su()),
		Audience:      app.NewGetAudienceBreakdown(fake, th(), su()),
		Funnel:        app.NewGetFunnel(fake),
		AdPerformance: app.NewGetAdPerformance(fake, th(), su()),
		ProposeStatus: app.NewProposeCampaignStatus(fake, store),
		Confirm:       app.NewConfirmProposal(store, &noopWriter{}, nil),
	})
	if srv == nil {
		t.Fatal("NewServer devolvió nil")
	}
}

func TestInsightsHandler_MalformedDateRejected(t *testing.T) {
	fake := &fakeReader{}
	h := insightsHandler(app.NewGetInsights(fake, th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{"since": "ayer", "until": "hoy"}))
	if !res.IsError {
		t.Fatal("expected error result for malformed dates")
	}
}
