package domain

// AdSetStatus es el estado configurado de un conjunto de anuncios. Meta usa el
// mismo vocabulario que para campañas.
type AdSetStatus = CampaignStatus

// AdSet es un conjunto de anuncios: la agrupación dentro de una campaña que
// define un público y que, cuando la campaña no administra el presupuesto de
// forma centralizada, tiene el suyo propio.
type AdSet struct {
	ID           string
	Name         string
	Status       AdSetStatus
	CampaignID   string
	CampaignName string

	// Budget es el presupuesto propio del conjunto. Si es nil, el presupuesto se
	// administra a nivel campaña y este conjunto no se puede ajustar por separado.
	Budget *Budget
}

// ManagesOwnBudget indica si el conjunto tiene presupuesto propio.
func (a AdSet) ManagesOwnBudget() bool { return a.Budget != nil }

// Editable indica si el conjunto admite cambios.
func (a AdSet) Editable() bool {
	return a.Status != CampaignArchived && a.Status != CampaignDeleted
}

// BudgetLevelError indica que se pidió escribir el presupuesto en el nivel
// equivocado. Cuando la plata vive en los conjuntos, transporta la lista para
// que la respuesta le muestre al usuario dónde elegir, en lugar de dejarlo con
// un error sin salida (FR-008).
type BudgetLevelError struct {
	CampaignID   string
	CampaignName string
	Expected     BudgetLevel // dónde se administra realmente el presupuesto
	AdSets       []AdSet     // poblado sólo si Expected es LevelAdSet
}

func (e *BudgetLevelError) Error() string {
	return "el presupuesto se administra a nivel " + string(e.Expected)
}

// Unwrap permite que errors.Is(err, ErrBudgetLevelMismatch) siga funcionando.
func (e *BudgetLevelError) Unwrap() error { return ErrBudgetLevelMismatch }
