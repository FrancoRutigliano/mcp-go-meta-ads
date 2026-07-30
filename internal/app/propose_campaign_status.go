package app

import (
	"context"
	"fmt"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// defaultProposalTTL es cuánto vive una propuesta antes de vencer.
const defaultProposalTTL = 5 * time.Minute

// ProposeCampaignStatus calcula (sin aplicar) el cambio de estado de una
// campaña y devuelve una propuesta. Es de SÓLO LECTURA (Principio II): lee el
// estado actual pero no toca Meta.
type ProposeCampaignStatus struct {
	reader ports.MetaReader
	store  ProposalStore
	now    func() time.Time
	ttl    time.Duration
}

// NewProposeCampaignStatus construye el caso de uso con reloj real y TTL default.
func NewProposeCampaignStatus(reader ports.MetaReader, store ProposalStore) *ProposeCampaignStatus {
	return NewProposeCampaignStatusWithClock(reader, store, time.Now, defaultProposalTTL)
}

// NewProposeCampaignStatusWithClock permite inyectar reloj y TTL (tests).
func NewProposeCampaignStatusWithClock(reader ports.MetaReader, store ProposalStore, now func() time.Time, ttl time.Duration) *ProposeCampaignStatus {
	return &ProposeCampaignStatus{reader: reader, store: store, now: now, ttl: ttl}
}

// Execute arma la propuesta de cambio de estado.
func (uc *ProposeCampaignStatus) Execute(ctx context.Context, campaignID string, action domain.CampaignAction) (domain.Proposal, error) {
	const op = "app.ProposeCampaignStatus"

	if !action.Valid() {
		return domain.Proposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("acción no soportada: %q", action))
	}
	if campaignID == "" {
		return domain.Proposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("falta el id de campaña"))
	}

	campaign, err := uc.reader.GetCampaign(ctx, campaignID)
	if err != nil {
		return domain.Proposal{}, err
	}

	target := action.TargetStatus()
	if campaign.Status == target {
		return domain.Proposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("%w (%s)", domain.ErrCampaignAlreadyInState, target))
	}

	now := uc.now()
	p := domain.Proposal{
		ID:           newProposalID(),
		Kind:         domain.ProposalCampaignStatus,
		CampaignID:   campaign.ID,
		CampaignName: campaign.Name,
		Field:        "estado",
		Before:       string(campaign.Status),
		After:        string(target),
		CreatedAt:    now,
		ExpiresAt:    now.Add(uc.ttl),
	}
	uc.store.Save(p)
	return p, nil
}
