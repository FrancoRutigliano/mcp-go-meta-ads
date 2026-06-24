package domain

// CampaignStatus es el estado configurado de una campaña en Meta.
type CampaignStatus string

const (
	CampaignActive   CampaignStatus = "ACTIVE"
	CampaignPaused   CampaignStatus = "PAUSED"
	CampaignArchived CampaignStatus = "ARCHIVED"
	CampaignDeleted  CampaignStatus = "DELETED"
)

// Campaign es una campaña publicitaria de la cuenta. Modela sólo lo que esta
// feature necesita: identidad, estado y objetivo.
type Campaign struct {
	ID        string
	Name      string
	Status    CampaignStatus
	Objective string
}

// CampaignQuery parametriza la lectura de campañas.
type CampaignQuery struct {
	OnlyActive bool // si true, sólo campañas con effective_status ACTIVE
	Limit      int  // tope de resultados; <=0 deja decidir un default al adapter
}
