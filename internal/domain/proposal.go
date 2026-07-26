package domain

import "time"

// ProposalKind identifica el tipo de cambio propuesto.
type ProposalKind string

const (
	// ProposalCampaignStatus: pausar o activar una campaña.
	ProposalCampaignStatus ProposalKind = "campaign_status"
)

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
}

// Expired indica si la propuesta ya venció (no puede confirmarse).
func (p Proposal) Expired(now time.Time) bool {
	return now.After(p.ExpiresAt)
}
