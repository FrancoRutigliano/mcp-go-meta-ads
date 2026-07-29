package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/adapters/memstore"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

func testGuardrails() domain.Guardrails {
	return domain.Guardrails{
		MaxIncreaseFactor: 3,
		MaxDailyBudget:    domain.MoneyFromPesos(25000),
	}
}

// campaignWithBudget arma una campaña que administra su propio presupuesto.
func campaignWithBudget(pesos float64) domain.Campaign {
	return domain.Campaign{
		ID:     "c1",
		Name:   "Ventas",
		Status: domain.CampaignActive,
		Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(pesos)},
	}
}

func amountChange(pesos float64) domain.BudgetChange {
	m := domain.MoneyFromPesos(pesos)
	return domain.BudgetChange{Amount: &m}
}

func percentChange(pct float64) domain.BudgetChange {
	return domain.BudgetChange{Percent: &pct}
}

func TestProposeBudget_AbsoluteAmount(t *testing.T) {
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	store := memstore.New()
	uc := NewProposeBudget(reader, store, testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	p, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     amountChange(5000),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if p.Proposal.Kind != domain.ProposalBudget {
		t.Errorf("Kind = %q, esperaba %q", p.Proposal.Kind, domain.ProposalBudget)
	}
	if p.Proposal.Level != domain.LevelCampaign {
		t.Errorf("Level = %q, esperaba %q", p.Proposal.Level, domain.LevelCampaign)
	}
	if p.Proposal.Budget == nil {
		t.Fatal("debe traer BudgetDelta")
	}
	if p.Proposal.Budget.Before.Pesos() != 3000 || p.Proposal.Budget.After.Pesos() != 5000 {
		t.Errorf("delta = %v → %v, esperaba 3000 → 5000", p.Proposal.Budget.Before.Pesos(), p.Proposal.Budget.After.Pesos())
	}
	if p.Proposal.EntityID != "c1" || p.Proposal.CampaignID != "c1" {
		t.Errorf("entidad = %q / campaña = %q, esperaba c1 en ambas", p.Proposal.EntityID, p.Proposal.CampaignID)
	}
	if _, ok := store.Get(p.Proposal.ID); !ok {
		t.Error("la propuesta debe quedar guardada en el store")
	}
}

func TestProposeBudget_RelativeChange(t *testing.T) {
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	p, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     percentChange(30),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	// El porcentaje se resuelve en el servidor: la propuesta lleva el monto final.
	if p.Proposal.Budget.After.Pesos() != 3900 {
		t.Errorf("After = %v, esperaba 3900 (3000 + 30%%)", p.Proposal.Budget.After.Pesos())
	}
}

func TestProposeBudget_NeverWrites(t *testing.T) {
	// El caso de uso ni siquiera recibe un writer: es estructuralmente incapaz
	// de escribir (Principio II).
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	if _, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     amountChange(5000),
	}); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !reader.called {
		t.Error("debe leer el presupuesto actual en el momento de proponer")
	}
}

func TestProposeBudget_Rejections(t *testing.T) {
	tests := []struct {
		name     string
		campaign domain.Campaign
		req      BudgetRequest
		wantKind domain.Kind
		wantErr  error
	}{
		{
			name:     "monto igual al actual",
			campaign: campaignWithBudget(3000),
			req:      BudgetRequest{CampaignID: "c1", Change: amountChange(3000)},
			wantKind: domain.KindInvalidInput,
			wantErr:  domain.ErrBudgetUnchanged,
		},
		{
			name:     "monto cero",
			campaign: campaignWithBudget(3000),
			req:      BudgetRequest{CampaignID: "c1", Change: amountChange(0)},
			wantKind: domain.KindInvalidInput,
		},
		{
			name:     "monto y porcentaje a la vez",
			campaign: campaignWithBudget(3000),
			req: BudgetRequest{CampaignID: "c1", Change: domain.BudgetChange{
				Amount:  moneyPtr(domain.MoneyFromPesos(5000)),
				Percent: pctPtr(30),
			}},
			wantKind: domain.KindInvalidInput,
			wantErr:  domain.ErrBothAmountAndPercent,
		},
		{
			name:     "sin indicar cambio",
			campaign: campaignWithBudget(3000),
			req:      BudgetRequest{CampaignID: "c1"},
			wantKind: domain.KindInvalidInput,
			wantErr:  domain.ErrNoBudgetChange,
		},
		{
			name: "campaña archivada",
			campaign: domain.Campaign{
				ID: "c1", Name: "Vieja", Status: domain.CampaignArchived,
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(3000)},
			},
			req:      BudgetRequest{CampaignID: "c1", Change: amountChange(5000)},
			wantKind: domain.KindInvalidInput,
		},
		{
			name:     "sin id de campaña",
			campaign: campaignWithBudget(3000),
			req:      BudgetRequest{Change: amountChange(5000)},
			wantKind: domain.KindInvalidInput,
		},
		{
			name:     "guardrail: salto mayor a 3x",
			campaign: campaignWithBudget(3000),
			req:      BudgetRequest{CampaignID: "c1", Change: amountChange(20000)},
			wantKind: domain.KindInvalidInput,
		},
		{
			name:     "guardrail: supera el techo diario",
			campaign: campaignWithBudget(20000),
			req:      BudgetRequest{CampaignID: "c1", Change: amountChange(30000)},
			wantKind: domain.KindInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &fakeReader{campaign: tt.campaign}
			store := memstore.New()
			uc := NewProposeBudget(reader, store, testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

			p, err := uc.Execute(context.Background(), tt.req)
			if err == nil {
				t.Fatalf("esperaba error, vino la propuesta %+v", p)
			}
			if domain.KindOf(err) != tt.wantKind {
				t.Errorf("Kind = %v, esperaba %v (err: %v)", domain.KindOf(err), tt.wantKind, err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, esperaba que envolviera %v", err, tt.wantErr)
			}
		})
	}
}

func TestProposeBudget_GuardrailErrorCarriesLimits(t *testing.T) {
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	_, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     amountChange(20000),
	})

	var ge *domain.GuardrailError
	if !errors.As(err, &ge) {
		t.Fatalf("esperaba *GuardrailError, vino %v", err)
	}
	if ge.Max.Pesos() != 9000 {
		t.Errorf("Max = %v, esperaba 9000 (3x de 3000)", ge.Max.Pesos())
	}
}

func TestProposeBudget_RejectsCampaignWithoutOwnBudget(t *testing.T) {
	// La campaña reparte la plata en sus conjuntos: escribir a nivel campaña
	// fallaría en Meta, así que se rechaza antes de llegar ahí (FR-007).
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSets: []domain.AdSet{
			{ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)}},
		},
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	_, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     amountChange(5000),
	})
	if !errors.Is(err, domain.ErrBudgetLevelMismatch) {
		t.Errorf("error = %v, esperaba que envolviera ErrBudgetLevelMismatch", err)
	}
}

func TestProposeBudget_ProposalExpires(t *testing.T) {
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	uc := NewProposeBudgetWithClock(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency(),
		func() time.Time { return now }, 5*time.Minute)

	p, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     amountChange(5000),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !p.Proposal.ExpiresAt.Equal(now.Add(5 * time.Minute)) {
		t.Errorf("ExpiresAt = %v, esperaba %v", p.Proposal.ExpiresAt, now.Add(5*time.Minute))
	}
}

func TestProposeBudget_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{err: domain.NewError(domain.KindNotFound, "meta", errors.New("no existe"))}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	_, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "inexistente",
		Change:     amountChange(5000),
	})
	if domain.KindOf(err) != domain.KindNotFound {
		t.Errorf("Kind = %v, esperaba not_found", domain.KindOf(err))
	}
}

func moneyPtr(m domain.Money) *domain.Money { return &m }
func pctPtr(p float64) *float64             { return &p }

func TestProposeBudget_CampaignLevelListsAdSets(t *testing.T) {
	// La plata está en los conjuntos: en vez de un error sin salida, la respuesta
	// devuelve la lista para que el usuario elija (FR-008).
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSets: []domain.AdSet{
			{ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)}},
			{ID: "as2", Name: "Remarketing", Status: domain.CampaignActive, CampaignID: "c1",
				Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(800)}},
		},
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	_, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		Change:     amountChange(5000),
	})

	var le *domain.BudgetLevelError
	if !errors.As(err, &le) {
		t.Fatalf("esperaba *BudgetLevelError, vino %v", err)
	}
	if le.Expected != domain.LevelAdSet {
		t.Errorf("Expected = %q, esperaba adset", le.Expected)
	}
	if len(le.AdSets) != 2 {
		t.Fatalf("esperaba los 2 conjuntos para elegir, vinieron %d", len(le.AdSets))
	}
	// Sigue siendo reconocible con errors.Is para el resto del sistema.
	if !errors.Is(err, domain.ErrBudgetLevelMismatch) {
		t.Error("debe seguir envolviendo ErrBudgetLevelMismatch")
	}
}

func TestProposeBudget_AdSetLevel(t *testing.T) {
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSet: domain.AdSet{
			ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
			Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)},
		},
	}
	store := memstore.New()
	uc := NewProposeBudget(reader, store, testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	p, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		AdSetID:    "as1",
		Change:     amountChange(2500),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if p.Proposal.Level != domain.LevelAdSet {
		t.Errorf("Level = %q, esperaba adset", p.Proposal.Level)
	}
	if p.Proposal.EntityID != "as1" || p.Proposal.EntityName != "Público frío" {
		t.Errorf("entidad = %q/%q, esperaba as1/Público frío", p.Proposal.EntityID, p.Proposal.EntityName)
	}
	// La campaña contenedora se conserva para la auditoría y la presentación.
	if p.Proposal.CampaignID != "c1" || p.Proposal.CampaignName != "Ventas" {
		t.Errorf("campaña = %q/%q, esperaba c1/Ventas", p.Proposal.CampaignID, p.Proposal.CampaignName)
	}
	if p.Proposal.Budget.Before.Pesos() != 1500 || p.Proposal.Budget.After.Pesos() != 2500 {
		t.Errorf("delta = %v → %v, esperaba 1500 → 2500", p.Proposal.Budget.Before.Pesos(), p.Proposal.Budget.After.Pesos())
	}
}

func TestProposeBudget_AdSetRejectedWhenCampaignControlsBudget(t *testing.T) {
	// El sentido inverso: la campaña administra la plata centralmente.
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	_, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1",
		AdSetID:    "as1",
		Change:     amountChange(2500),
	})

	var le *domain.BudgetLevelError
	if !errors.As(err, &le) {
		t.Fatalf("esperaba *BudgetLevelError, vino %v", err)
	}
	if le.Expected != domain.LevelCampaign {
		t.Errorf("Expected = %q, esperaba campaign", le.Expected)
	}
}

func TestProposeBudget_AdSetInactiveCampaignMeansNoSpend(t *testing.T) {
	// Conjunto activo en campaña pausada: el cambio no genera gasto todavía.
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignPaused},
		adSet: domain.AdSet{
			ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
			Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)},
		},
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	p, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", AdSetID: "as1", Change: amountChange(2500),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if p.Proposal.Active {
		t.Error("con la campaña pausada, el cambio no debería marcarse como gasto inmediato")
	}
}

func TestProposeBudget_CampaignWithoutBudgetNorAdSets(t *testing.T) {
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(), domain.DefaultThresholds(), domain.DefaultSufficiency())

	_, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", Change: amountChange(5000),
	})
	if !errors.Is(err, domain.ErrNoAdSets) {
		t.Errorf("error = %v, esperaba que envolviera ErrNoAdSets", err)
	}
}

// insightWithROAS arma un insight de campaña con el ROAS y las compras pedidas.
func insightWithROAS(roas float64, purchases int64) domain.Insight {
	r, p := roas, purchases
	return domain.Insight{
		CampaignID:   "c1",
		CampaignName: "Ventas",
		Metrics: domain.Metrics{
			Spend: 10000, Impressions: 50000, Clicks: 800, LinkClicks: 600,
			ROAS: &r, Purchases: &p,
		},
	}
}

func TestProposeBudget_AttachesPerformance(t *testing.T) {
	reader := &fakeReader{
		campaign: campaignWithBudget(3000),
		insights: []domain.Insight{insightWithROAS(6.39, 22)},
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(),
		domain.DefaultThresholds(), domain.DefaultSufficiency())

	bp, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", Change: amountChange(5000),
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if bp.Metrics == nil {
		t.Fatal("la propuesta debe traer el rendimiento reciente")
	}
	if bp.Eval.ROAS != domain.StatusOK {
		t.Errorf("ROAS 6,39x debería evaluarse OK contra el mínimo de 2x, vino %v", bp.Eval.ROAS)
	}
	if bp.Eval.Insufficient {
		t.Error("con 22 compras hay muestra suficiente")
	}
	if bp.Period.Days() <= 0 {
		t.Error("debe informar el período usado")
	}
}

func TestProposeBudget_FlagsLowROAS(t *testing.T) {
	reader := &fakeReader{
		campaign: campaignWithBudget(3000),
		insights: []domain.Insight{insightWithROAS(0.9, 20)},
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(),
		domain.DefaultThresholds(), domain.DefaultSufficiency())

	bp, _ := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", Change: amountChange(5000),
	})

	if bp.Eval.ROAS != domain.StatusBad {
		t.Errorf("ROAS 0,9x debería marcarse como bajo rendimiento, vino %v", bp.Eval.ROAS)
	}
	// Pero la propuesta se arma igual: advertir no es bloquear (FR-025).
	if bp.Proposal.ID == "" {
		t.Error("la propuesta debe existir aunque el rendimiento sea malo")
	}
}

func TestProposeBudget_InsufficientDataDoesNotBlock(t *testing.T) {
	reader := &fakeReader{
		campaign: campaignWithBudget(3000),
		insights: []domain.Insight{insightWithROAS(8.0, 0)}, // sin compras en el período
	}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(),
		domain.DefaultThresholds(), domain.DefaultSufficiency())

	bp, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", Change: amountChange(5000),
	})
	if err != nil {
		t.Fatalf("datos insuficientes no deben bloquear la propuesta: %v", err)
	}
	if !bp.Eval.Insufficient {
		t.Error("con 1 compra la muestra debería marcarse insuficiente")
	}
	if bp.Proposal.ID == "" {
		t.Error("la propuesta debe existir igual")
	}
}

func TestProposeBudget_PerformanceFailureIsReportedNotHidden(t *testing.T) {
	// El reader falla al leer insights, pero la campaña se leyó bien: la
	// propuesta se arma y se avisa que falta el dato (Principios V y IX).
	reader := &failingInsightsReader{campaign: campaignWithBudget(3000)}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(),
		domain.DefaultThresholds(), domain.DefaultSufficiency())

	bp, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", Change: amountChange(5000),
	})
	if err != nil {
		t.Fatalf("no debería fallar la propuesta entera: %v", err)
	}
	if !bp.PerfErr {
		t.Error("debe marcar que no se pudo leer el rendimiento")
	}
	if bp.Metrics != nil {
		t.Error("no debe inventar métricas")
	}
}

// failingInsightsReader lee campañas bien pero falla al pedir rendimiento.
type failingInsightsReader struct {
	fakeReader
	campaign domain.Campaign
}

func (f *failingInsightsReader) GetCampaign(context.Context, string) (domain.Campaign, error) {
	return f.campaign, nil
}

func (f *failingInsightsReader) GetInsights(context.Context, domain.InsightQuery) ([]domain.Insight, error) {
	return nil, domain.NewError(domain.KindRateLimited, "meta", errors.New("slow down"))
}

func TestProposeBudget_MakesAtMostThreeMetaCalls(t *testing.T) {
	// El paso propose no debe degradar el rate limiting (Principio VI).
	reader := &countingReader{campaign: campaignWithBudget(3000)}
	uc := NewProposeBudget(reader, memstore.New(), testGuardrails(),
		domain.DefaultThresholds(), domain.DefaultSufficiency())

	if _, err := uc.Execute(context.Background(), BudgetRequest{
		CampaignID: "c1", Change: amountChange(5000),
	}); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if reader.calls > 3 {
		t.Errorf("hizo %d llamadas a Meta, el máximo aceptable es 3", reader.calls)
	}
}

// countingReader cuenta las llamadas salientes.
type countingReader struct {
	fakeReader
	campaign domain.Campaign
	calls    int
}

func (c *countingReader) GetCampaign(context.Context, string) (domain.Campaign, error) {
	c.calls++
	return c.campaign, nil
}

func (c *countingReader) GetInsights(context.Context, domain.InsightQuery) ([]domain.Insight, error) {
	c.calls++
	return nil, nil
}

func (c *countingReader) GetAdSets(context.Context, string) ([]domain.AdSet, error) {
	c.calls++
	return nil, nil
}

func (c *countingReader) GetAdSet(context.Context, string) (domain.AdSet, error) {
	c.calls++
	return domain.AdSet{}, nil
}
