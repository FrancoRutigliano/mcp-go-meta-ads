package app

import (
	"context"
	"errors"
	"testing"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func adSet(id, name string, pesos float64, status domain.AdSetStatus) domain.AdSet {
	set := domain.AdSet{ID: id, Name: name, Status: status, CampaignID: "c1"}
	if pesos > 0 {
		set.Budget = &domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(pesos)}
	}
	return set
}

func TestGetBudgets_CampaignLevel(t *testing.T) {
	reader := &fakeReader{campaign: campaignWithBudget(3000)}
	uc := NewGetBudgets(reader)

	ov, err := uc.Execute(context.Background(), "c1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if ov.Level != domain.LevelCampaign {
		t.Errorf("Level = %q, esperaba campaign", ov.Level)
	}
	if ov.Campaign == nil || ov.Campaign.Amount.Pesos() != 3000 {
		t.Errorf("presupuesto de campaña = %+v, esperaba 3000", ov.Campaign)
	}
	if len(ov.AdSets) != 0 {
		t.Errorf("no debería listar conjuntos cuando la plata está en la campaña")
	}
	if ov.TotalDaily.Pesos() != 3000 {
		t.Errorf("TotalDaily = %v, esperaba 3000", ov.TotalDaily.Pesos())
	}
}

func TestGetBudgets_AdSetLevel(t *testing.T) {
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
		adSets: []domain.AdSet{
			adSet("as1", "Público frío", 1500, domain.CampaignActive),
			adSet("as2", "Remarketing", 800, domain.CampaignPaused),
		},
	}
	uc := NewGetBudgets(reader)

	ov, err := uc.Execute(context.Background(), "c1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if ov.Level != domain.LevelAdSet {
		t.Errorf("Level = %q, esperaba adset", ov.Level)
	}
	if ov.Campaign != nil {
		t.Error("no debería traer presupuesto de campaña")
	}
	if len(ov.AdSets) != 2 {
		t.Fatalf("esperaba 2 conjuntos, vinieron %d", len(ov.AdSets))
	}
	// El total diario suma sólo lo que está efectivamente gastando.
	if ov.TotalDaily.Pesos() != 1500 {
		t.Errorf("TotalDaily = %v, esperaba 1500 (sólo el conjunto activo)", ov.TotalDaily.Pesos())
	}
}

func TestGetBudgets_CampaignWithoutAdSets(t *testing.T) {
	// Sin presupuesto propio y sin conjuntos: se dice explícitamente, no se
	// devuelve una lista vacía que parezca un resultado (FR-010, Principio IX).
	reader := &fakeReader{
		campaign: domain.Campaign{ID: "c1", Name: "Ventas", Status: domain.CampaignActive},
	}
	uc := NewGetBudgets(reader)

	_, err := uc.Execute(context.Background(), "c1")
	if !errors.Is(err, domain.ErrNoAdSets) {
		t.Errorf("error = %v, esperaba que envolviera ErrNoAdSets", err)
	}
}

func TestGetBudgets_RequiresCampaignID(t *testing.T) {
	uc := NewGetBudgets(&fakeReader{})

	_, err := uc.Execute(context.Background(), "")
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("Kind = %v, esperaba invalid_input", domain.KindOf(err))
	}
}

func TestGetBudgets_PropagatesReaderError(t *testing.T) {
	reader := &fakeReader{err: domain.NewError(domain.KindNotFound, "meta", errors.New("no existe"))}
	uc := NewGetBudgets(reader)

	_, err := uc.Execute(context.Background(), "c1")
	if domain.KindOf(err) != domain.KindNotFound {
		t.Errorf("Kind = %v, esperaba not_found", domain.KindOf(err))
	}
}
