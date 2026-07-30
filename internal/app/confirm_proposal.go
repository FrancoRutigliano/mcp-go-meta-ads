package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// ConfirmProposal ejecuta una propuesta previamente generada (Principio II). Es
// el ÚNICO camino de escritura, y registra la acción en el log de auditoría
// (Principio IV).
type ConfirmProposal struct {
	store  ProposalStore
	reader ports.MetaReader
	writer ports.MetaWriter
	now    func() time.Time
	audit  *slog.Logger
}

// NewConfirmProposal construye el caso de uso con reloj real. El reader se usa
// para releer el estado actual antes de escribir y detectar que la base cambió
// desde que se armó la propuesta.
func NewConfirmProposal(store ProposalStore, reader ports.MetaReader, writer ports.MetaWriter, audit *slog.Logger) *ConfirmProposal {
	return NewConfirmProposalWithClock(store, reader, writer, audit, time.Now)
}

// NewConfirmProposalWithClock permite inyectar reloj (tests).
func NewConfirmProposalWithClock(store ProposalStore, reader ports.MetaReader, writer ports.MetaWriter, audit *slog.Logger, now func() time.Time) *ConfirmProposal {
	if audit == nil {
		audit = slog.Default()
	}
	return &ConfirmProposal{store: store, reader: reader, writer: writer, audit: audit, now: now}
}

// Execute aplica la propuesta identificada por proposalID. confirmedBy queda
// registrado en la auditoría (quién confirmó).
func (uc *ConfirmProposal) Execute(ctx context.Context, proposalID, confirmedBy string) (domain.Proposal, error) {
	const op = "app.ConfirmProposal"

	if proposalID == "" {
		return domain.Proposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("falta el id de propuesta"))
	}

	p, ok := uc.store.Get(proposalID)
	if !ok {
		return domain.Proposal{}, domain.NewError(domain.KindNotFound, op,
			fmt.Errorf("%w: %q", domain.ErrProposalNotFound, proposalID))
	}
	if p.Expired(uc.now()) {
		uc.store.Delete(p.ID)
		return domain.Proposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("%w: %q", domain.ErrProposalExpired, proposalID))
	}

	if confirmedBy == "" {
		confirmedBy = "usuario MCP"
	}

	if err := uc.apply(ctx, p); err != nil {
		// No consumimos la propuesta si falló: permite reintentar.
		return domain.Proposal{}, err
	}

	// Consumo de un solo uso: no se puede re-confirmar.
	uc.store.Delete(p.ID)

	// Auditoría (Principio IV): timestamp lo agrega slog automáticamente.
	level := p.Level
	if level == "" {
		level = domain.LevelCampaign
	}
	uc.audit.Info("escritura confirmada",
		"accion", string(p.Kind),
		"nivel", string(level),
		"entidad_id", p.TargetID(),
		"campaña_id", p.CampaignID,
		"campaña", p.CampaignName,
		"campo", p.Field,
		"antes", p.Before,
		"despues", p.After,
		"confirmado_por", confirmedBy,
		"propuesta_id", p.ID,
	)
	return p, nil
}

func (uc *ConfirmProposal) apply(ctx context.Context, p domain.Proposal) error {
	const op = "app.ConfirmProposal"

	switch p.Kind {
	case domain.ProposalCampaignStatus:
		return uc.writer.UpdateCampaignStatus(ctx, p.CampaignID, domain.CampaignStatus(p.After))
	case domain.ProposalBudget:
		return uc.applyBudget(ctx, p)
	default:
		return domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("tipo de propuesta no soportado: %q", p.Kind))
	}
}

// applyBudget relee el presupuesto vigente antes de escribir. Si cambió desde
// que se armó la propuesta (por ejemplo, alguien lo tocó desde el Business
// Manager), no se aplica: el usuario aprobó un cambio sobre una base que ya no
// existe (FR-030, decisión D6 del plan).
func (uc *ConfirmProposal) applyBudget(ctx context.Context, p domain.Proposal) error {
	const op = "app.ConfirmProposal"

	if p.Budget == nil {
		return domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("la propuesta de presupuesto no trae el cambio calculado"))
	}

	current, err := uc.currentBudget(ctx, p)
	if err != nil {
		return err
	}
	if current.Amount.Cents != p.Budget.Before.Cents || current.Type != p.Budget.Type {
		return domain.NewError(domain.KindInvalidInput, op, domain.ErrBudgetDrifted)
	}

	next := domain.Budget{Type: p.Budget.Type, Amount: p.Budget.After}

	// Se escribe sólo sobre la entidad de la propuesta: un cambio en un conjunto
	// no toca a los demás conjuntos de la campaña (FR-031).
	if p.Level == domain.LevelAdSet {
		return uc.writer.UpdateAdSetBudget(ctx, p.TargetID(), next)
	}
	return uc.writer.UpdateCampaignBudget(ctx, p.TargetID(), next)
}

// currentBudget lee el presupuesto vigente de la entidad afectada.
func (uc *ConfirmProposal) currentBudget(ctx context.Context, p domain.Proposal) (domain.Budget, error) {
	const op = "app.ConfirmProposal"

	if p.Level == domain.LevelAdSet {
		set, err := uc.reader.GetAdSet(ctx, p.TargetID())
		if err != nil {
			return domain.Budget{}, err
		}
		if !set.ManagesOwnBudget() {
			// El presupuesto se movió de nivel entre el propose y el confirm.
			return domain.Budget{}, domain.NewError(domain.KindInvalidInput, op, domain.ErrBudgetDrifted)
		}
		return *set.Budget, nil
	}

	campaign, err := uc.reader.GetCampaign(ctx, p.CampaignID)
	if err != nil {
		return domain.Budget{}, err
	}
	if !campaign.ManagesOwnBudget() {
		return domain.Budget{}, domain.NewError(domain.KindInvalidInput, op, domain.ErrBudgetDrifted)
	}
	return *campaign.Budget, nil
}
