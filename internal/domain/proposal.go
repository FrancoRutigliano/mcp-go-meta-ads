package domain

import "time"

// ProposalKind identifica el tipo de cambio propuesto.
type ProposalKind string

const (
	// ProposalCampaignStatus: pausar o activar una campaña.
	ProposalCampaignStatus ProposalKind = "campaign_status"
	// ProposalBudget: cambiar el monto de presupuesto de una campaña o de un
	// conjunto de anuncios.
	ProposalBudget ProposalKind = "budget"
)

// BudgetDelta es el cambio de presupuesto calculado, con los montos tipados. Se
// guarda aparte de los campos Before/After (que son de presentación) porque el
// confirm necesita comparar el valor numérico para detectar deriva.
type BudgetDelta struct {
	Type   BudgetType
	Before Money
	After  Money
}

// CampaignAction es la acción pedida sobre una campaña.
type CampaignAction string

const (
	ActionPause    CampaignAction = "pause"
	ActionActivate CampaignAction = "activate"
)

// Valid indica si la acción está soportada.
func (a CampaignAction) Valid() bool {
	return a == ActionPause || a == ActionActivate
}

// TargetStatus es el estado resultante de aplicar la acción.
func (a CampaignAction) TargetStatus() CampaignStatus {
	if a == ActionActivate {
		return CampaignActive
	}
	return CampaignPaused
}

// Proposal es un cambio de escritura calculado pero NO aplicado (Principio II:
// propose es de sólo lectura). El confirm posterior lo ejecuta por su ID.
type Proposal struct {
	ID           string
	Kind         ProposalKind
	CampaignID   string
	CampaignName string
	Field        string // qué cambia, legible (ej: "estado")
	Before       string // valor actual
	After        string // valor propuesto
	CreatedAt    time.Time
	ExpiresAt    time.Time

	// Campos de la feature 010. Son opcionales: las propuestas de estado los
	// dejan en su valor cero y no cambian de forma.
	Level      BudgetLevel  // dónde se aplica: campaña o conjunto de anuncios
	EntityID   string       // id de la entidad afectada (== CampaignID si Level es campaña)
	EntityName string       // nombre de la entidad afectada
	Budget     *BudgetDelta // nil salvo que Kind sea ProposalBudget
	Active     bool         // la entidad está activa: el cambio genera gasto ya (FR-004)
}

// TargetID devuelve el id de la entidad sobre la que se aplica el cambio.
// Para las propuestas de estado (009) siempre es la campaña.
func (p Proposal) TargetID() string {
	if p.EntityID != "" {
		return p.EntityID
	}
	return p.CampaignID
}

// Expired indica si la propuesta ya venció (no puede confirmarse).
func (p Proposal) Expired(now time.Time) bool {
	return now.After(p.ExpiresAt)
}
