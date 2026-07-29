package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// BudgetRequest es el pedido de cambio de presupuesto que llega desde la tool.
// Identifica la entidad afectada y la forma del cambio.
type BudgetRequest struct {
	CampaignID string // campaña a ajustar, o campaña contenedora del conjunto
	AdSetID    string // conjunto de anuncios a ajustar; vacío = nivel campaña
	Change     domain.BudgetChange
}

// BudgetProposal es la propuesta más el contexto de rendimiento que acompaña la
// decisión. La evaluación vive acá y no en el dominio porque es una lectura
// interpretada, no parte del cambio a aplicar.
type BudgetProposal struct {
	Proposal domain.Proposal
	Metrics  *domain.Metrics // nil si no se pudo leer el rendimiento
	Eval     Evaluation
	Period   domain.DateRange
	PerfErr  bool // no se pudo leer el rendimiento; se dice, no se oculta
}

// ProposeBudget calcula (sin aplicar) un cambio de presupuesto y devuelve la
// propuesta. Es de SÓLO LECTURA: no recibe un ports.MetaWriter, así que es
// estructuralmente incapaz de escribir (Constitución, Principio II).
type ProposeBudget struct {
	reader     ports.MetaReader
	store      ProposalStore
	guardrails domain.Guardrails
	th         domain.Thresholds
	su         domain.SufficiencyPolicy
	now        func() time.Time
	ttl        time.Duration
}

// NewProposeBudget construye el caso de uso con reloj real y TTL default.
func NewProposeBudget(reader ports.MetaReader, store ProposalStore, g domain.Guardrails, th domain.Thresholds, su domain.SufficiencyPolicy) *ProposeBudget {
	return NewProposeBudgetWithClock(reader, store, g, th, su, time.Now, defaultProposalTTL)
}

// NewProposeBudgetWithClock permite inyectar reloj y TTL (tests).
func NewProposeBudgetWithClock(reader ports.MetaReader, store ProposalStore, g domain.Guardrails, th domain.Thresholds, su domain.SufficiencyPolicy, now func() time.Time, ttl time.Duration) *ProposeBudget {
	return &ProposeBudget{reader: reader, store: store, guardrails: g, th: th, su: su, now: now, ttl: ttl}
}

// Execute arma la propuesta de cambio de presupuesto con su contexto de
// rendimiento.
func (uc *ProposeBudget) Execute(ctx context.Context, req BudgetRequest) (BudgetProposal, error) {
	const op = "app.ProposeBudget"

	if req.CampaignID == "" && req.AdSetID == "" {
		return BudgetProposal{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("falta indicar la campaña o el conjunto de anuncios"))
	}
	// Validar el pedido antes de salir a la red: un pedido mal formado no
	// justifica gastar una llamada contra Meta.
	if err := req.Change.Validate(); err != nil {
		return BudgetProposal{}, err
	}

	target, err := uc.resolveTarget(ctx, req)
	if err != nil {
		return BudgetProposal{}, err
	}

	proposed, err := req.Change.Apply(target.budget.Amount)
	if err != nil {
		return BudgetProposal{}, err
	}
	if err := uc.guardrails.Check(target.budget.Amount, proposed, target.budget.Type); err != nil {
		return BudgetProposal{}, err
	}

	now := uc.now()
	p := domain.Proposal{
		ID:           newProposalID(),
		Kind:         domain.ProposalBudget,
		CampaignID:   target.campaignID,
		CampaignName: target.campaignName,
		Level:        target.level,
		EntityID:     target.entityID,
		EntityName:   target.entityName,
		Field:        "presupuesto",
		Before:       pesosText(target.budget.Amount),
		After:        pesosText(proposed),
		Budget: &domain.BudgetDelta{
			Type:   target.budget.Type,
			Before: target.budget.Amount,
			After:  proposed,
		},
		Active:    target.active,
		CreatedAt: now,
		ExpiresAt: now.Add(uc.ttl),
	}
	uc.store.Save(p)

	out := BudgetProposal{Proposal: p}
	uc.attachPerformance(ctx, &out)
	return out, nil
}

// attachPerformance agrega el rendimiento reciente de la campaña para que la
// decisión sea informada (FR-023 a FR-025). Si no se puede leer, se marca y se
// dice en la respuesta: la propuesta sigue siendo válida, pero el usuario tiene
// que saber que decide sin ese dato (Principios V y IX).
func (uc *ProposeBudget) attachPerformance(ctx context.Context, out *BudgetProposal) {
	until := truncateToDay(uc.now().UTC())
	period := domain.DateRange{
		Since: until.AddDate(0, 0, -defaultInsightDays),
		Until: until,
	}
	out.Period = period

	insights, err := uc.reader.GetInsights(ctx, domain.InsightQuery{
		CampaignID: out.Proposal.CampaignID,
		Range:      period,
	})
	if err != nil || len(insights) == 0 {
		out.PerfErr = err != nil
		return
	}

	// A nivel campaña la Graph API devuelve una fila; si viniera más de una, la
	// primera es la de la campaña pedida.
	m := insights[0].Metrics
	out.Metrics = &m
	out.Eval = Evaluate(m, period.Days(), uc.th, uc.su)
}

// budgetTarget es la entidad resuelta sobre la que se aplicará el cambio.
type budgetTarget struct {
	level        domain.BudgetLevel
	entityID     string
	entityName   string
	campaignID   string
	campaignName string
	budget       domain.Budget
	active       bool
}

// resolveTarget determina sobre qué entidad se aplica el cambio y verifica que
// el presupuesto realmente se administre en ese nivel (FR-007). Rechaza el nivel
// equivocado en los dos sentidos, antes de llegar a Meta.
func (uc *ProposeBudget) resolveTarget(ctx context.Context, req BudgetRequest) (budgetTarget, error) {
	const op = "app.ProposeBudget"

	campaign, err := uc.reader.GetCampaign(ctx, req.CampaignID)
	if err != nil {
		return budgetTarget{}, err
	}
	if !campaign.Editable() {
		return budgetTarget{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("la campaña \"%s\" está %s y no admite cambios de presupuesto",
				campaign.Name, campaign.Status))
	}

	if req.AdSetID != "" {
		return uc.resolveAdSet(ctx, campaign, req.AdSetID)
	}

	if !campaign.ManagesOwnBudget() {
		// La plata está en los conjuntos: en vez de dejar al usuario con un error
		// sin salida, se los devolvemos para que elija (FR-008).
		return budgetTarget{}, uc.levelMismatch(ctx, campaign)
	}

	return budgetTarget{
		level:        domain.LevelCampaign,
		entityID:     campaign.ID,
		entityName:   campaign.Name,
		campaignID:   campaign.ID,
		campaignName: campaign.Name,
		budget:       *campaign.Budget,
		active:       campaign.Status == domain.CampaignActive,
	}, nil
}

// resolveAdSet valida y resuelve el conjunto de anuncios pedido.
func (uc *ProposeBudget) resolveAdSet(ctx context.Context, campaign domain.Campaign, adSetID string) (budgetTarget, error) {
	const op = "app.ProposeBudget"

	if campaign.ManagesOwnBudget() {
		// El caso inverso: la campaña controla la plata de forma centralizada, así
		// que ajustar un conjunto suelto no es posible.
		return budgetTarget{}, domain.NewError(domain.KindInvalidInput, op, &domain.BudgetLevelError{
			CampaignID:   campaign.ID,
			CampaignName: campaign.Name,
			Expected:     domain.LevelCampaign,
		})
	}

	set, err := uc.reader.GetAdSet(ctx, adSetID)
	if err != nil {
		return budgetTarget{}, err
	}
	if !set.Editable() {
		return budgetTarget{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("el conjunto \"%s\" está %s y no admite cambios de presupuesto",
				set.Name, set.Status))
	}
	if !set.ManagesOwnBudget() {
		return budgetTarget{}, domain.NewError(domain.KindInvalidInput, op, &domain.BudgetLevelError{
			CampaignID:   campaign.ID,
			CampaignName: campaign.Name,
			Expected:     domain.LevelCampaign,
		})
	}

	return budgetTarget{
		level:        domain.LevelAdSet,
		entityID:     set.ID,
		entityName:   set.Name,
		campaignID:   campaign.ID,
		campaignName: campaign.Name,
		budget:       *set.Budget,
		active:       set.Status == domain.CampaignActive && campaign.Status == domain.CampaignActive,
	}, nil
}

// levelMismatch arma el error de nivel equivocado con la lista de conjuntos.
func (uc *ProposeBudget) levelMismatch(ctx context.Context, campaign domain.Campaign) error {
	const op = "app.ProposeBudget"

	sets, err := uc.reader.GetAdSets(ctx, campaign.ID)
	if err != nil {
		// Si no podemos listar los conjuntos, igual hay que decir que el nivel es
		// el equivocado: silenciarlo sería peor.
		return domain.NewError(domain.KindInvalidInput, op, domain.ErrBudgetLevelMismatch)
	}
	if len(sets) == 0 {
		return domain.NewError(domain.KindNotFound, op, domain.ErrNoAdSets)
	}

	for i := range sets {
		sets[i].CampaignName = campaign.Name
	}
	return domain.NewError(domain.KindInvalidInput, op, &domain.BudgetLevelError{
		CampaignID:   campaign.ID,
		CampaignName: campaign.Name,
		Expected:     domain.LevelAdSet,
		AdSets:       sets,
	})
}

// pesosText representa el monto en pesos enteros, para los campos legibles de la
// propuesta y el log de auditoría.
func pesosText(m domain.Money) string {
	return strconv.FormatInt(int64(m.Pesos()), 10)
}
