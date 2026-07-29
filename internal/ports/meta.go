// Package ports define las interfaces (puertos) del dominio hacia el exterior.
//
// Constitución, Principio III: el server es la única frontera con Meta. El
// dominio y los casos de uso dependen sólo de esta abstracción; el adaptador
// concreto que habla con la Graph API vive detrás de MetaReader y es el único
// que conoce el token y el cliente HTTP.
package ports

import (
	"context"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

// MetaReader expone las operaciones de SÓLO LECTURA contra Meta.
type MetaReader interface {
	// ListCampaigns devuelve las campañas de la cuenta configurada.
	ListCampaigns(ctx context.Context, q domain.CampaignQuery) ([]domain.Campaign, error)

	// GetCampaign devuelve una campaña puntual (para calcular el estado actual
	// en el paso propose).
	GetCampaign(ctx context.Context, id string) (domain.Campaign, error)

	// GetInsights devuelve el rendimiento por campaña para el período pedido.
	GetInsights(ctx context.Context, q domain.InsightQuery) ([]domain.Insight, error)

	// GetAudienceBreakdown devuelve el rendimiento segmentado por una dimensión
	// (edad, género, región, plataforma, posición) para el período pedido.
	GetAudienceBreakdown(ctx context.Context, q domain.AudienceQuery) (domain.AudienceBreakdown, error)

	// GetAdInsights devuelve el rendimiento por anuncio (creativo) para el
	// período pedido.
	GetAdInsights(ctx context.Context, q domain.AdQuery) ([]domain.AdInsight, error)

	// GetAdSets devuelve los conjuntos de anuncios de una campaña con su
	// presupuesto, para resolver dónde se administra la plata.
	GetAdSets(ctx context.Context, campaignID string) ([]domain.AdSet, error)

	// GetAdSet devuelve un conjunto de anuncios puntual.
	GetAdSet(ctx context.Context, id string) (domain.AdSet, error)
}

// MetaWriter expone las operaciones de ESCRITURA contra Meta. Está separado de
// MetaReader a propósito: sólo el caso de uso de confirm (tras un propose
// válido) lo usa, nunca las tools de lectura (Constitución, Principios II y III).
type MetaWriter interface {
	// UpdateCampaignStatus pausa o activa una campaña (operación irreversible
	// sobre el estado real).
	UpdateCampaignStatus(ctx context.Context, campaignID string, status domain.CampaignStatus) error

	// UpdateCampaignBudget fija el presupuesto de una campaña que administra su
	// propio presupuesto. Mueve gasto real: sólo el confirm puede invocarlo.
	UpdateCampaignBudget(ctx context.Context, campaignID string, budget domain.Budget) error

	// UpdateAdSetBudget fija el presupuesto de un conjunto de anuncios. Mueve
	// gasto real: sólo el confirm puede invocarlo.
	UpdateAdSetBudget(ctx context.Context, adSetID string, budget domain.Budget) error
}
