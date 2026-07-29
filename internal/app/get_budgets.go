package app

import (
	"context"
	"fmt"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// BudgetOverview responde "¿dónde está la plata de esta campaña y cuánta hay?".
// Es de sólo lectura: sirve para decidir antes de proponer un cambio (FR-009).
type BudgetOverview struct {
	CampaignID   string
	CampaignName string
	Level        domain.BudgetLevel // dónde se administra el presupuesto
	Campaign     *domain.Budget     // presupuesto propio, si Level es campaña
	AdSets       []domain.AdSet     // conjuntos con su presupuesto, si Level es conjunto
	TotalDaily   domain.Money       // gasto diario comprometido hoy
}

// GetBudgets lee el presupuesto vigente de una campaña y, si corresponde, el de
// sus conjuntos de anuncios.
type GetBudgets struct {
	reader ports.MetaReader
}

// NewGetBudgets construye el caso de uso.
func NewGetBudgets(reader ports.MetaReader) *GetBudgets {
	return &GetBudgets{reader: reader}
}

// Execute resuelve el panorama de presupuesto de la campaña.
func (uc *GetBudgets) Execute(ctx context.Context, campaignID string) (BudgetOverview, error) {
	const op = "app.GetBudgets"

	if campaignID == "" {
		return BudgetOverview{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("falta el id de campaña"))
	}

	campaign, err := uc.reader.GetCampaign(ctx, campaignID)
	if err != nil {
		return BudgetOverview{}, err
	}

	ov := BudgetOverview{
		CampaignID:   campaign.ID,
		CampaignName: campaign.Name,
	}

	if campaign.ManagesOwnBudget() {
		ov.Level = domain.LevelCampaign
		ov.Campaign = campaign.Budget
		if campaign.Budget.Type == domain.BudgetDaily && campaign.Status == domain.CampaignActive {
			ov.TotalDaily = campaign.Budget.Amount
		}
		return ov, nil
	}

	sets, err := uc.reader.GetAdSets(ctx, campaign.ID)
	if err != nil {
		return BudgetOverview{}, err
	}
	if len(sets) == 0 {
		return BudgetOverview{}, domain.NewError(domain.KindNotFound, op, domain.ErrNoAdSets)
	}

	ov.Level = domain.LevelAdSet
	ov.AdSets = make([]domain.AdSet, 0, len(sets))
	for _, set := range sets {
		// El nombre de la campaña no viene en la respuesta de conjuntos, pero es
		// lo que el usuario reconoce, así que se completa acá.
		set.CampaignName = campaign.Name
		ov.AdSets = append(ov.AdSets, set)

		// Sólo suma lo que está efectivamente gastando: un conjunto pausado tiene
		// presupuesto configurado pero no consume.
		if set.ManagesOwnBudget() && set.Budget.Type == domain.BudgetDaily && set.Status == domain.CampaignActive {
			ov.TotalDaily.Cents += set.Budget.Amount.Cents
		}
	}
	return ov, nil
}
