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

// MetaReader expone las operaciones de SÓLO LECTURA contra Meta que esta
// feature necesita. No incluye ninguna operación de escritura (las features de
// escritura agregarán su propio puerto con el flujo propose/confirm).
type MetaReader interface {
	// ListCampaigns devuelve las campañas de la cuenta configurada.
	ListCampaigns(ctx context.Context, q domain.CampaignQuery) ([]domain.Campaign, error)

	// GetInsights devuelve el rendimiento por campaña para el período pedido.
	GetInsights(ctx context.Context, q domain.InsightQuery) ([]domain.Insight, error)

	// GetAudienceBreakdown devuelve el rendimiento segmentado por una dimensión
	// (edad, género, región, plataforma, posición) para el período pedido.
	GetAudienceBreakdown(ctx context.Context, q domain.AudienceQuery) (domain.AudienceBreakdown, error)
}
