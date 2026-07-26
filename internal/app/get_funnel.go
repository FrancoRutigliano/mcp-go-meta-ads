package app

import (
	"context"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

// FunnelStep es un paso del embudo. Known=false significa que el pixel no lo
// registra (distinto de Count=0).
type FunnelStep struct {
	Name  string
	Count int64
	Known bool
}

// CampaignFunnel es el embudo de una campaña con el análisis de dónde se cae.
type CampaignFunnel struct {
	CampaignID   string
	CampaignName string
	Steps        []FunnelStep
	// Mayor caída entre pasos conocidos posteriores al clic.
	HasLeak     bool
	LeakFrom    string
	LeakTo      string
	LeakPct     float64 // porcentaje perdido en ese paso (0..100)
	PixelGaps   bool    // faltan pasos intermedios (pixel incompleto)
	AnyActivity bool    // hubo al menos impresiones
}

// GetFunnel es el caso de uso que arma el embudo de conversión por campaña.
type GetFunnel struct {
	reader ports.MetaReader
	now    func() time.Time
}

// NewGetFunnel construye el caso de uso con reloj real.
func NewGetFunnel(reader ports.MetaReader) *GetFunnel {
	return NewGetFunnelWithClock(reader, time.Now)
}

// NewGetFunnelWithClock permite inyectar un reloj (tests).
func NewGetFunnelWithClock(reader ports.MetaReader, now func() time.Time) *GetFunnel {
	return &GetFunnel{reader: reader, now: now}
}

// Execute arma el embudo por campaña. campaignID vacío consulta toda la cuenta.
func (uc *GetFunnel) Execute(ctx context.Context, campaignID string, rng *domain.DateRange) ([]CampaignFunnel, domain.DateRange, error) {
	applied := resolveRangeWith(uc.now, rng)
	if err := applied.Valid(); err != nil {
		return nil, domain.DateRange{}, domain.NewError(domain.KindInvalidInput, "app.GetFunnel", err)
	}

	insights, err := uc.reader.GetInsights(ctx, domain.InsightQuery{CampaignID: campaignID, Range: applied})
	if err != nil {
		return nil, domain.DateRange{}, err
	}

	out := make([]CampaignFunnel, 0, len(insights))
	for _, in := range insights {
		out = append(out, buildFunnel(in.CampaignID, in.CampaignName, in.Metrics))
	}
	return out, applied, nil
}

// buildFunnel arma los pasos y detecta la mayor caída del embudo on-site (desde
// el clic en adelante; el salto impresiones→clic siempre es enorme y normal).
func buildFunnel(id, name string, m domain.Metrics) CampaignFunnel {
	steps := []FunnelStep{
		{Name: "Impresiones", Count: m.Impressions, Known: true},
		{Name: "Clics de enlace", Count: m.LinkClicks, Known: true},
		{Name: "Vistas de página", Count: optI(m.LandingPageViews), Known: m.LandingPageViews != nil},
		{Name: "Agregar al carrito", Count: optI(m.AddToCart), Known: m.AddToCart != nil},
		{Name: "Iniciar pago", Count: optI(m.InitiateCheckout), Known: m.InitiateCheckout != nil},
		{Name: "Compras", Count: optI(m.Purchases), Known: m.Purchases != nil},
	}

	f := CampaignFunnel{
		CampaignID:   id,
		CampaignName: name,
		Steps:        steps,
		PixelGaps:    m.LandingPageViews == nil || m.AddToCart == nil || m.InitiateCheckout == nil,
		AnyActivity:  m.Impressions > 0,
	}

	// Mayor caída entre pasos conocidos, empezando desde "Clics de enlace" (idx 1).
	for i := 1; i < len(steps)-1; i++ {
		a, b := steps[i], steps[i+1]
		if !a.Known || !b.Known || a.Count <= 0 {
			continue
		}
		pct := (1 - float64(b.Count)/float64(a.Count)) * 100
		if pct > f.LeakPct {
			f.LeakPct = pct
			f.LeakFrom = a.Name
			f.LeakTo = b.Name
			f.HasLeak = true
		}
	}
	return f
}

func optI(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// resolveRangeWith aplica la ventana por defecto (últimos 30 días) si rng es nil.
func resolveRangeWith(now func() time.Time, rng *domain.DateRange) domain.DateRange {
	if rng != nil {
		return *rng
	}
	until := truncateToDay(now().UTC())
	since := until.AddDate(0, 0, -defaultInsightDays)
	return domain.DateRange{Since: since, Until: until}
}
