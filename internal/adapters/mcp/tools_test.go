package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

type fakeReader struct {
	campaigns []domain.Campaign
	insights  []domain.Insight
	breakdown domain.AudienceBreakdown
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

func TestInsightsHandler_MalformedDateRejected(t *testing.T) {
	fake := &fakeReader{}
	h := insightsHandler(app.NewGetInsights(fake, th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{"since": "ayer", "until": "hoy"}))
	if !res.IsError {
		t.Fatal("expected error result for malformed dates")
	}
}
