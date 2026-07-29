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
	budget domain.Budget
	err    error
}

func (w *noopWriter) UpdateCampaignBudget(_ context.Context, _ string, b domain.Budget) error {
	w.calls++
	w.budget = b
	return w.err
}

func (w *noopWriter) UpdateAdSetBudget(_ context.Context, _ string, b domain.Budget) error {
	w.calls++
	w.budget = b
	return w.err
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
	adSets    []domain.AdSet
	adSet     domain.AdSet
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
func (f *fakeReader) GetAdSets(context.Context, string) ([]domain.AdSet, error) {
	return f.adSets, f.err
}
func (f *fakeReader) GetAdSet(context.Context, string) (domain.AdSet, error) {
	return f.adSet, f.err
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
	confirm := confirmActionHandler(app.NewConfirmProposal(store, fake, writer, nil))

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
	confirm := confirmActionHandler(app.NewConfirmProposal(memstore.New(), &fakeReader{}, &noopWriter{}, nil))
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
		"get_budgets":             budgetsTool().Name,
		"propose_campaign_status": proposeCampaignStatusTool().Name,
		"propose_budget":          proposeBudgetTool().Name,
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
		Budgets:       app.NewGetBudgets(fake),
		ProposeStatus: app.NewProposeCampaignStatus(fake, store),
		ProposeBudget: app.NewProposeBudget(fake, store, domain.DefaultGuardrails(), th(), su()),
		Confirm:       app.NewConfirmProposal(store, fake, &noopWriter{}, nil),
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

// Principio II: confirm_action debe ser la única tool marcada como destructiva.
// Cualquier propose_* que se anote como destructiva sería una señal equivocada
// para el cliente MCP.
func TestOnlyConfirmActionIsDestructive(t *testing.T) {
	tools := []mcp.Tool{
		campaignsTool(), insightsTool(), audienceTool(), funnelTool(), adPerformanceTool(), budgetsTool(),
		proposeCampaignStatusTool(), proposeBudgetTool(), confirmActionTool(),
	}

	for _, tool := range tools {
		destructive := tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint
		if tool.Name == "confirm_action" {
			if !destructive {
				t.Error("confirm_action debe estar anotada como destructiva")
			}
			continue
		}
		if destructive {
			t.Errorf("%s no debería estar anotada como destructiva", tool.Name)
		}
	}
}

func TestProposeBudgetHandler_ProposesWithoutWriting(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{
		ID: "c1", Name: "Ventas", Status: domain.CampaignActive,
		Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(3000)},
	}}
	store := memstore.New()
	h := proposeBudgetHandler(app.NewProposeBudget(fake, store, domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id": "c1",
		"amount_ars":  5000.0,
	}))

	if res.IsError {
		t.Fatalf("no esperaba error: %+v", res)
	}
	text := resultText(res)
	for _, want := range []string{"todavía no cambié nada", "$3.000", "$5.000", "confirm_action"} {
		if !strings.Contains(text, want) {
			t.Errorf("la propuesta debería mencionar %q, vino: %s", want, text)
		}
	}
}

func TestProposeBudgetHandler_RequiresAChange(t *testing.T) {
	fake := &fakeReader{}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{"campaign_id": "c1"}))

	if !res.IsError {
		t.Fatal("sin monto ni porcentaje debería ser error")
	}
}

func TestProposeBudgetHandler_GuardrailMessageIsHuman(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{
		ID: "c1", Name: "Ventas", Status: domain.CampaignActive,
		Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1000)},
	}}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id": "c1",
		"amount_ars":  15000.0, // 15x: excede el factor de 3x
	}))

	if !res.IsError {
		t.Fatal("un salto de 15x debería rechazarse")
	}
	if text := resultText(res); !strings.Contains(text, "$3.000") {
		t.Errorf("el mensaje debería decir el máximo admitido, vino: %s", text)
	}
}

func TestBudgetsHandler_AdSetLevelListsThem(t *testing.T) {
	fake := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSets: []domain.AdSet{
			{ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)}},
			{ID: "as2", Name: "Remarketing", Status: domain.CampaignPaused, CampaignID: "c1",
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(800)}},
		},
	}
	h := budgetsHandler(app.NewGetBudgets(fake))

	res, _ := h(context.Background(), newRequest(map[string]any{"campaign_id": "c1"}))

	if res.IsError {
		t.Fatalf("no esperaba error: %s", resultText(res))
	}
	text := resultText(res)
	for _, want := range []string{"Público frío", "Remarketing", "$1.500", "$800", "pausada", "as1"} {
		if !strings.Contains(text, want) {
			t.Errorf("la salida debería mencionar %q, vino: %s", want, text)
		}
	}
	// Sólo el conjunto activo cuenta como gasto comprometido.
	if !strings.Contains(text, "Gasto diario comprometido: $1.500") {
		t.Errorf("el total diario debería ser $1.500, vino: %s", text)
	}
}

func TestBudgetsHandler_CampaignLevel(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{
		ID: "c1", Name: "Ventas", Status: domain.CampaignActive,
		Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(3000)},
	}}
	h := budgetsHandler(app.NewGetBudgets(fake))

	res, _ := h(context.Background(), newRequest(map[string]any{"campaign_id": "c1"}))

	text := resultText(res)
	if !strings.Contains(text, "a nivel campaña") || !strings.Contains(text, "$3.000") {
		t.Errorf("salida inesperada: %s", text)
	}
	// $3.000/día ≈ $90.000/mes.
	if !strings.Contains(text, "$90.000") {
		t.Errorf("debería proyectar el gasto mensual, vino: %s", text)
	}
}

func TestProposeBudgetHandler_LevelMismatchListsAdSets(t *testing.T) {
	// El caso más importante de UX: pedir el cambio en la campaña cuando la plata
	// está en los conjuntos no deja al usuario sin salida (FR-008).
	fake := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSets: []domain.AdSet{
			{ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)}},
		},
	}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id": "c1",
		"amount_ars":  5000.0,
	}))

	if !res.IsError {
		t.Fatal("debería rechazarse: el presupuesto está en los conjuntos")
	}
	text := resultText(res)
	for _, want := range []string{"Público frío", "as1", "adset_id"} {
		if !strings.Contains(text, want) {
			t.Errorf("el mensaje debería guiar al usuario con %q, vino: %s", want, text)
		}
	}
}

func TestProposeBudgetHandler_AdSetProposal(t *testing.T) {
	fake := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSet: domain.AdSet{
			ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
			Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)},
		},
	}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id":    "c1",
		"adset_id":       "as1",
		"percent_change": 30.0,
	}))

	if res.IsError {
		t.Fatalf("no esperaba error: %s", resultText(res))
	}
	text := resultText(res)
	for _, want := range []string{"conjunto de anuncios", "Público frío", "Ventas", "$1.500", "$1.950", "+30%"} {
		if !strings.Contains(text, want) {
			t.Errorf("la propuesta debería mencionar %q, vino: %s", want, text)
		}
	}
}

func TestProposeBudgetHandler_ShowsPerformanceContext(t *testing.T) {
	roas := 6.39
	purchases := int64(22)
	fake := &fakeReader{
		campaign: domain.Campaign{
			ID: "c1", Name: "Ventas", Status: domain.CampaignActive,
			Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(3000)},
		},
		insights: []domain.Insight{{
			CampaignID: "c1", CampaignName: "Ventas",
			Metrics: domain.Metrics{
				Spend: 10000, Impressions: 50000, Clicks: 800, LinkClicks: 600,
				ROAS: &roas, Purchases: &purchases,
			},
		}},
	}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id": "c1", "amount_ars": 5000.0,
	}))

	text := resultText(res)
	for _, want := range []string{"Rendimiento", "6.39x", "✅", "mínimo de 2x"} {
		if !strings.Contains(text, want) {
			t.Errorf("la propuesta debería mostrar %q, vino: %s", want, text)
		}
	}
}

func TestProposeBudgetHandler_WarnsOnLowROAS(t *testing.T) {
	roas := 0.9
	purchases := int64(20)
	fake := &fakeReader{
		campaign: domain.Campaign{
			ID: "c1", Name: "Ventas", Status: domain.CampaignActive,
			Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(3000)},
		},
		insights: []domain.Insight{{
			CampaignID: "c1",
			Metrics: domain.Metrics{
				Spend: 10000, Impressions: 50000, Clicks: 800, LinkClicks: 600,
				ROAS: &roas, Purchases: &purchases,
			},
		}},
	}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id": "c1", "amount_ars": 5000.0,
	}))

	if res.IsError {
		t.Fatal("advertir no es bloquear: la propuesta debe armarse igual")
	}
	if text := resultText(res); !strings.Contains(text, "por debajo del mínimo de 2x") {
		t.Errorf("debería advertir el bajo rendimiento, vino: %s", text)
	}
}

func TestProposeBudgetHandler_WarnsOnInsufficientData(t *testing.T) {
	fake := &fakeReader{
		campaign: domain.Campaign{
			ID: "c1", Name: "Ventas", Status: domain.CampaignActive,
			Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(3000)},
		},
		// Sin insights: no hay actividad sobre la cual apoyarse.
	}
	h := proposeBudgetHandler(app.NewProposeBudget(fake, memstore.New(), domain.DefaultGuardrails(), th(), su()))

	res, _ := h(context.Background(), newRequest(map[string]any{
		"campaign_id": "c1", "amount_ars": 5000.0,
	}))

	if res.IsError {
		t.Fatal("la falta de datos no debe bloquear la propuesta")
	}
	if text := resultText(res); !strings.Contains(text, "Sin actividad registrada") {
		t.Errorf("debería declarar que no hay datos, vino: %s", text)
	}
}
