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
// feature necesita: identidad, estado, objetivo y dónde vive su presupuesto.
type Campaign struct {
	ID        string
	Name      string
	Status    CampaignStatus
	Objective string

	// Budget es el presupuesto a nivel campaña. Si es nil, la campaña NO
	// administra su propio presupuesto: la plata está repartida en sus conjuntos
	// de anuncios. Es la señal que usa el paso propose para resolver el nivel.
	Budget *Budget
}

// ManagesOwnBudget indica si el presupuesto se controla a nivel campaña.
func (c Campaign) ManagesOwnBudget() bool { return c.Budget != nil }

// Editable indica si la campaña admite cambios. Las archivadas y eliminadas no.
func (c Campaign) Editable() bool {
	return c.Status != CampaignArchived && c.Status != CampaignDeleted
}

// CampaignQuery parametriza la lectura de campañas.
type CampaignQuery struct {
	OnlyActive bool // si true, sólo campañas con effective_status ACTIVE
	Limit      int  // tope de resultados; <=0 deja decidir un default al adapter
}
