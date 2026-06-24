// Package app contiene los casos de uso. Orquestan el dominio y dependen sólo
// de los puertos, nunca de adaptadores concretos (arquitectura hexagonal).
package app

import (
	"context"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// DefaultCampaignLimit acota la respuesta cuando el llamador no fija un tope.
// La cuenta puede tener cientos de campañas (FR-012), por eso nunca se lista
// sin límite.
const DefaultCampaignLimit = 50

// ListCampaigns es el caso de uso para listar campañas.
type ListCampaigns struct {
	reader ports.MetaReader
}

// NewListCampaigns construye el caso de uso con su dependencia (el puerto).
func NewListCampaigns(reader ports.MetaReader) *ListCampaigns {
	return &ListCampaigns{reader: reader}
}

// Execute devuelve las campañas de la cuenta. onlyActive filtra por estado
// activo; limit<=0 aplica el tope por defecto.
func (uc *ListCampaigns) Execute(ctx context.Context, onlyActive bool, limit int) ([]domain.Campaign, error) {
	if limit <= 0 {
		limit = DefaultCampaignLimit
	}
	return uc.reader.ListCampaigns(ctx, domain.CampaignQuery{
		OnlyActive: onlyActive,
		Limit:      limit,
	})
}
