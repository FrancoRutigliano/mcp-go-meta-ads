package domain

import (
	"testing"
	"time"
)

func TestCampaignAction(t *testing.T) {
	if !ActionPause.Valid() || !ActionActivate.Valid() {
		t.Error("pause/activate deben ser válidas")
	}
	if CampaignAction("delete").Valid() {
		t.Error("delete no debe ser válida")
	}
	if ActionPause.TargetStatus() != CampaignPaused {
		t.Error("pause → PAUSED")
	}
	if ActionActivate.TargetStatus() != CampaignActive {
		t.Error("activate → ACTIVE")
	}
}

func TestProposal_Expired(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	p := Proposal{ExpiresAt: now.Add(5 * time.Minute)}

	if p.Expired(now) {
		t.Error("no debería estar vencida antes de ExpiresAt")
	}
	if !p.Expired(now.Add(6 * time.Minute)) {
		t.Error("debería estar vencida después de ExpiresAt")
	}
}

func TestProposal_BudgetKind(t *testing.T) {
	p := Proposal{
		ID:           "prop_x",
		Kind:         ProposalBudget,
		CampaignID:   "c1",
		CampaignName: "Ventas",
		Level:        LevelAdSet,
		EntityID:     "as1",
		EntityName:   "Público frío",
		Field:        "presupuesto",
		Budget: &BudgetDelta{
			Type:   BudgetDaily,
			Before: MoneyFromPesos(1500),
			After:  MoneyFromPesos(2500),
		},
	}

	if p.Kind != ProposalBudget {
		t.Errorf("Kind = %q, esperaba %q", p.Kind, ProposalBudget)
	}
	if p.Budget == nil {
		t.Fatal("una propuesta de presupuesto debe traer BudgetDelta")
	}
	if p.Budget.Before.Pesos() != 1500 || p.Budget.After.Pesos() != 2500 {
		t.Errorf("BudgetDelta = %v → %v, esperaba 1500 → 2500",
			p.Budget.Before.Pesos(), p.Budget.After.Pesos())
	}
	// La entidad afectada es el conjunto, pero la campaña contenedora se conserva
	// para la auditoría (FR-029).
	if p.EntityID != "as1" || p.CampaignID != "c1" {
		t.Error("debe conservar tanto la entidad afectada como su campaña")
	}
}

func TestProposal_StatusKindStaysCompatible(t *testing.T) {
	// Las propuestas de estado de la 009 se siguen construyendo igual: los campos
	// nuevos son opcionales y quedan en su valor cero.
	p := Proposal{
		Kind:       ProposalCampaignStatus,
		CampaignID: "c1",
		Field:      "estado",
		Before:     string(CampaignActive),
		After:      string(CampaignPaused),
	}

	if p.Budget != nil {
		t.Error("una propuesta de estado no debería traer BudgetDelta")
	}
	if p.Level != "" || p.EntityID != "" {
		t.Error("los campos de nivel/entidad deberían quedar vacíos")
	}
}

func TestCampaign_BudgetLevelSignal(t *testing.T) {
	// Budget == nil significa que la plata vive en los conjuntos (decisión D2).
	sinPropio := Campaign{ID: "c1", Name: "Ventas"}
	if sinPropio.ManagesOwnBudget() {
		t.Error("sin Budget, la campaña no administra su propio presupuesto")
	}

	conPropio := Campaign{
		ID:     "c2",
		Name:   "Remarketing",
		Budget: &Budget{Type: BudgetDaily, Amount: MoneyFromPesos(3000)},
	}
	if !conPropio.ManagesOwnBudget() {
		t.Error("con Budget, la campaña administra su propio presupuesto")
	}
}
