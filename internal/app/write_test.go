package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/adapters/memstore"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

type fakeWriter struct {
	calls     int
	gotID     string
	gotStatus domain.CampaignStatus
	gotBudget domain.Budget
	err       error
}

func (w *fakeWriter) UpdateCampaignStatus(_ context.Context, id string, s domain.CampaignStatus) error {
	w.calls++
	w.gotID = id
	w.gotStatus = s
	return w.err
}

func (w *fakeWriter) UpdateAdSetBudget(_ context.Context, id string, b domain.Budget) error {
	w.calls++
	w.gotID = id
	w.gotBudget = b
	return w.err
}

func (w *fakeWriter) UpdateCampaignBudget(_ context.Context, id string, b domain.Budget) error {
	w.calls++
	w.gotID = id
	w.gotBudget = b
	return w.err
}

func TestProposeCampaignStatus_BuildsProposalWithoutWriting(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{ID: "1", Name: "Ventas", Status: domain.CampaignActive}}
	store := memstore.New()
	uc := NewProposeCampaignStatus(fake, store)

	p, err := uc.Execute(context.Background(), "1", domain.ActionPause)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Before != "ACTIVE" || p.After != "PAUSED" {
		t.Errorf("propuesta mal armada: %+v", p)
	}
	if _, ok := store.Get(p.ID); !ok {
		t.Error("la propuesta debe quedar guardada en el store")
	}
}

func TestProposeCampaignStatus_RejectsNoOp(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{ID: "1", Status: domain.CampaignPaused}}
	uc := NewProposeCampaignStatus(fake, memstore.New())

	_, err := uc.Execute(context.Background(), "1", domain.ActionPause) // ya está pausada
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("pausar algo ya pausado debe ser invalid_input, got %v", err)
	}
}

func TestConfirmProposal_AppliesAndAudits(t *testing.T) {
	store := memstore.New()
	writer := &fakeWriter{}
	var buf bytes.Buffer
	audit := slog.New(slog.NewTextHandler(&buf, nil))
	uc := NewConfirmProposal(store, &fakeReader{}, writer, audit)

	now := time.Now()
	p := domain.Proposal{
		ID: "prop_x", Kind: domain.ProposalCampaignStatus, CampaignID: "1", CampaignName: "Ventas",
		Field: "estado", Before: "ACTIVE", After: "PAUSED", ExpiresAt: now.Add(5 * time.Minute),
	}
	store.Save(p)

	out, err := uc.Execute(context.Background(), "prop_x", "Mariana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer.calls != 1 || writer.gotStatus != domain.CampaignPaused {
		t.Errorf("no aplicó el cambio correcto: %+v", writer)
	}
	if _, ok := store.Get("prop_x"); ok {
		t.Error("la propuesta debe consumirse (un solo uso)")
	}
	if out.CampaignName != "Ventas" {
		t.Errorf("debe devolver la propuesta aplicada")
	}
	log := buf.String()
	for _, want := range []string{"escritura confirmada", "antes=ACTIVE", "despues=PAUSED", "confirmado_por=Mariana"} {
		if !strings.Contains(log, want) {
			t.Errorf("auditoría sin %q en: %s", want, log)
		}
	}
}

// Principio II: no se puede confirmar algo que no fue propuesto.
func TestConfirmProposal_UnknownIDRejected(t *testing.T) {
	uc := NewConfirmProposal(memstore.New(), &fakeReader{}, &fakeWriter{}, slog.Default())
	_, err := uc.Execute(context.Background(), "prop_inventado", "x")
	if domain.KindOf(err) != domain.KindNotFound {
		t.Errorf("id inexistente debe ser not_found, got %v", err)
	}
}

func TestConfirmProposal_ExpiredRejected(t *testing.T) {
	store := memstore.New()
	store.Save(domain.Proposal{ID: "prop_v", Kind: domain.ProposalCampaignStatus, ExpiresAt: time.Now().Add(-time.Minute)})
	writer := &fakeWriter{}
	uc := NewConfirmProposal(store, &fakeReader{}, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_v", "x")
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("propuesta vencida debe ser invalid_input, got %v", err)
	}
	if writer.calls != 0 {
		t.Error("no debe escribir si la propuesta venció")
	}
}

func TestConfirmProposal_KeepsProposalOnWriteFailure(t *testing.T) {
	store := memstore.New()
	store.Save(domain.Proposal{
		ID: "prop_e", Kind: domain.ProposalCampaignStatus, CampaignID: "1",
		After: "PAUSED", ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	writer := &fakeWriter{err: domain.NewError(domain.KindUnauthorized, "meta", errors.New("solo lectura"))}
	uc := NewConfirmProposal(store, &fakeReader{}, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_e", "x")
	if domain.KindOf(err) != domain.KindUnauthorized {
		t.Errorf("debe propagar el error de escritura, got %v", err)
	}
	if _, ok := store.Get("prop_e"); !ok {
		t.Error("si falla la escritura, la propuesta NO debe consumirse (para reintentar)")
	}
}

// budgetProposal arma una propuesta de presupuesto vigente sobre una campaña.
func budgetProposal(id string, before, after float64) domain.Proposal {
	return domain.Proposal{
		ID:           id,
		Kind:         domain.ProposalBudget,
		CampaignID:   "c1",
		CampaignName: "Ventas",
		Level:        domain.LevelCampaign,
		EntityID:     "c1",
		EntityName:   "Ventas",
		Field:        "presupuesto",
		Before:       pesosText(domain.MoneyFromPesos(before)),
		After:        pesosText(domain.MoneyFromPesos(after)),
		Budget: &domain.BudgetDelta{
			Type:   domain.BudgetDaily,
			Before: domain.MoneyFromPesos(before),
			After:  domain.MoneyFromPesos(after),
		},
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

func TestConfirmProposal_AppliesBudgetAndAudits(t *testing.T) {
	store := memstore.New()
	store.Save(budgetProposal("prop_b", 3000, 5000))
	writer := &fakeWriter{}
	reader := &fakeReader{campaign: campaignWithBudget(3000)} // la base no cambió
	var buf bytes.Buffer
	audit := slog.New(slog.NewTextHandler(&buf, nil))
	uc := NewConfirmProposal(store, reader, writer, audit)

	_, err := uc.Execute(context.Background(), "prop_b", "Mariana")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if writer.calls != 1 {
		t.Fatalf("esperaba 1 escritura, hubo %d", writer.calls)
	}
	if writer.gotID != "c1" {
		t.Errorf("escribió sobre %q, esperaba c1", writer.gotID)
	}
	if writer.gotBudget.Amount.Pesos() != 5000 || writer.gotBudget.Type != domain.BudgetDaily {
		t.Errorf("presupuesto aplicado = %+v, esperaba 5000 diario", writer.gotBudget)
	}
	if _, ok := store.Get("prop_b"); ok {
		t.Error("la propuesta debe consumirse (un solo uso)")
	}

	log := buf.String()
	for _, want := range []string{"escritura confirmada", "nivel=campaign", "entidad_id=c1", "confirmado_por=Mariana"} {
		if !strings.Contains(log, want) {
			t.Errorf("auditoría sin %q en: %s", want, log)
		}
	}
}

func TestConfirmProposal_RejectsDriftedBudget(t *testing.T) {
	// Alguien cambió el presupuesto desde el Business Manager entre el propose y
	// el confirm: la base ya no es la que se le mostró al usuario (FR-030).
	store := memstore.New()
	store.Save(budgetProposal("prop_d", 3000, 5000))
	writer := &fakeWriter{}
	reader := &fakeReader{campaign: campaignWithBudget(4200)} // derivó
	uc := NewConfirmProposal(store, reader, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_d", "Mariana")

	if !errors.Is(err, domain.ErrBudgetDrifted) {
		t.Errorf("error = %v, esperaba que envolviera ErrBudgetDrifted", err)
	}
	if writer.calls != 0 {
		t.Error("no debe escribir si la base cambió")
	}
	if _, ok := store.Get("prop_d"); !ok {
		t.Error("la propuesta no debe consumirse: el usuario puede volver a proponer")
	}
}

func TestConfirmProposal_KeepsBudgetProposalOnWriteFailure(t *testing.T) {
	store := memstore.New()
	store.Save(budgetProposal("prop_f", 3000, 5000))
	writer := &fakeWriter{err: domain.NewError(domain.KindUnauthorized, "meta", errors.New("sin ads_management"))}
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewConfirmProposal(store, reader, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_f", "x")

	if domain.KindOf(err) != domain.KindUnauthorized {
		t.Errorf("debe propagar el error de escritura, got %v", err)
	}
	if _, ok := store.Get("prop_f"); !ok {
		t.Error("si falla la escritura, la propuesta NO debe consumirse")
	}
}

func TestConfirmProposal_BudgetProposalCannotBeConfirmedTwice(t *testing.T) {
	store := memstore.New()
	store.Save(budgetProposal("prop_2x", 3000, 5000))
	writer := &fakeWriter{}
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewConfirmProposal(store, reader, writer, slog.Default())

	if _, err := uc.Execute(context.Background(), "prop_2x", "x"); err != nil {
		t.Fatalf("primera confirmación falló: %v", err)
	}
	_, err := uc.Execute(context.Background(), "prop_2x", "x")
	if domain.KindOf(err) != domain.KindNotFound {
		t.Errorf("la segunda confirmación debe ser not_found, got %v", err)
	}
	if writer.calls != 1 {
		t.Errorf("escribió %d veces, esperaba 1", writer.calls)
	}
}

func TestConfirmProposal_AppliesAdSetBudgetOnly(t *testing.T) {
	// Confirmar sobre un conjunto escribe en el nodo del conjunto, nunca en la
	// campaña ni en los otros conjuntos (FR-031).
	store := memstore.New()
	p := budgetProposal("prop_as", 1500, 2500)
	p.Level = domain.LevelAdSet
	p.EntityID = "as1"
	p.EntityName = "Público frío"
	store.Save(p)

	writer := &fakeWriter{}
	reader := &fakeReader{adSet: domain.AdSet{
		ID: "as1", Name: "Público frío", Status: domain.CampaignActive, CampaignID: "c1",
		Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1500)},
	}}
	var buf bytes.Buffer
	uc := NewConfirmProposal(store, reader, writer, slog.New(slog.NewTextHandler(&buf, nil)))

	if _, err := uc.Execute(context.Background(), "prop_as", "Mariana"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if writer.gotID != "as1" {
		t.Errorf("escribió sobre %q, esperaba as1", writer.gotID)
	}
	if writer.gotBudget.Amount.Pesos() != 2500 {
		t.Errorf("monto aplicado = %v, esperaba 2500", writer.gotBudget.Amount.Pesos())
	}
	if log := buf.String(); !strings.Contains(log, "nivel=adset") || !strings.Contains(log, "entidad_id=as1") {
		t.Errorf("la auditoría debe identificar el conjunto: %s", log)
	}
}

func TestConfirmProposal_AdSetDriftRejected(t *testing.T) {
	store := memstore.New()
	p := budgetProposal("prop_asd", 1500, 2500)
	p.Level = domain.LevelAdSet
	p.EntityID = "as1"
	store.Save(p)

	writer := &fakeWriter{}
	reader := &fakeReader{adSet: domain.AdSet{
		ID: "as1", Status: domain.CampaignActive, CampaignID: "c1",
		Budget: &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(1900)}, // derivó
	}}
	uc := NewConfirmProposal(store, reader, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_asd", "x")
	if !errors.Is(err, domain.ErrBudgetDrifted) {
		t.Errorf("error = %v, esperaba ErrBudgetDrifted", err)
	}
	if writer.calls != 0 {
		t.Error("no debe escribir si la base cambió")
	}
}

func TestConfirmProposal_IndependentProposalsDoNotInterfere(t *testing.T) {
	// Reasignación entre campañas: bajar en una y subir en otra son dos
	// operaciones separadas; confirmar una no arrastra a la otra (US4).
	store := memstore.New()

	baja := budgetProposal("prop_baja", 5000, 2000)
	suba := budgetProposal("prop_suba", 3000, 6000)
	suba.CampaignID = "c2"
	suba.EntityID = "c2"
	suba.CampaignName = "Remarketing"
	store.Save(baja)
	store.Save(suba)

	writer := &fakeWriter{}
	reader := &fakeReader{campaign: campaignWithBudget(5000)}
	uc := NewConfirmProposal(store, reader, writer, slog.Default())

	if _, err := uc.Execute(context.Background(), "prop_baja", "Mariana"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if writer.calls != 1 {
		t.Errorf("escribió %d veces, esperaba 1", writer.calls)
	}
	if writer.gotBudget.Amount.Pesos() != 2000 {
		t.Errorf("aplicó %v, esperaba la baja a 2000", writer.gotBudget.Amount.Pesos())
	}
	if _, ok := store.Get("prop_suba"); !ok {
		t.Error("la otra propuesta debe seguir pendiente e intacta")
	}
}
