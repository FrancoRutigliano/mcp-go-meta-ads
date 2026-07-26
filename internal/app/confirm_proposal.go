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
	writer ports.MetaWriter
	now    func() time.Time
	audit  *slog.Logger
}

// NewConfirmProposal construye el caso de uso con reloj real.
func NewConfirmProposal(store ProposalStore, writer ports.MetaWriter, audit *slog.Logger) *ConfirmProposal {
	return NewConfirmProposalWithClock(store, writer, audit, time.Now)
}

// NewConfirmProposalWithClock permite inyectar reloj (tests).
func NewConfirmProposalWithClock(store ProposalStore, writer ports.MetaWriter, audit *slog.Logger, now func() time.Time) *ConfirmProposal {
	if audit == nil {
		audit = slog.Default()
	}
	return &ConfirmProposal{store: store, writer: writer, audit: audit, now: now}
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
			fmt.Errorf("propuesta %q inexistente", proposalID))
	}
	if p.Expired(uc.now()) {
		uc.store.Delete(p.ID)
		return domain.Proposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("la propuesta %q venció", proposalID))
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
	uc.audit.Info("escritura confirmada",
		"accion", string(p.Kind),
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
	switch p.Kind {
	case domain.ProposalCampaignStatus:
		return uc.writer.UpdateCampaignStatus(ctx, p.CampaignID, domain.CampaignStatus(p.After))
	default:
		return domain.NewError(domain.KindInvalidInput, "app.ConfirmProposal",
			fmt.Errorf("tipo de propuesta no soportado: %q", p.Kind))
	}
}
