package meta

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

// dataEnvelope es el sobre estándar { "data": [...] } de la Graph API.
type dataEnvelope[T any] struct {
	Data []T `json:"data"`
}

type rawCampaign struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Objective string `json:"objective"`
}

// actionValue es el objeto que Meta usa dentro de arrays como actions,
// action_values, cost_per_action_type y purchase_roas.
type actionValue struct {
	ActionType string `json:"action_type"`
	Value      string `json:"value"`
}

type rawInsight struct {
	CampaignID   string `json:"campaign_id"`
	CampaignName string `json:"campaign_name"`
	Spend        string `json:"spend"`
	Impressions  string `json:"impressions"`
	Clicks       string `json:"clicks"`
	Reach        string `json:"reach"`
	CTR          string `json:"ctr"`
	CPC          string `json:"cpc"`
	DateStart    string `json:"date_start"`
	DateStop     string `json:"date_stop"`

	// Conversión / enlace / entrega (feature 002).
	Frequency          string        `json:"frequency"`
	InlineLinkClicks   string        `json:"inline_link_clicks"`
	InlineLinkClickCTR string        `json:"inline_link_click_ctr"`
	PurchaseROAS       []actionValue `json:"purchase_roas"`
	Actions            []actionValue `json:"actions"`
	ActionValues       []actionValue `json:"action_values"`
	CostPerActionType  []actionValue `json:"cost_per_action_type"`

	// Claves de segmento (presentes según el breakdown pedido).
	Age               string `json:"age"`
	Gender            string `json:"gender"`
	Region            string `json:"region"`
	PublisherPlatform string `json:"publisher_platform"`
	PlatformPosition  string `json:"platform_position"`
}

const graphDateLayout = "2006-01-02"

// purchaseActionTypes lista, por orden de prioridad, los action_type que Meta
// usa para "compra". Se toma el primero presente para evitar doble conteo.
var purchaseActionTypes = []string{
	"purchase",
	"omni_purchase",
	"offsite_conversion.fb_pixel_purchase",
	"onsite_web_purchase",
	"onsite_web_app_purchase",
}

// parseCampaigns convierte el JSON de campañas en entidades del dominio.
func parseCampaigns(body []byte) ([]domain.Campaign, error) {
	var env dataEnvelope[rawCampaign]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	out := make([]domain.Campaign, 0, len(env.Data))
	for _, rc := range env.Data {
		out = append(out, domain.Campaign{
			ID:        rc.ID,
			Name:      rc.Name,
			Status:    domain.CampaignStatus(rc.Status),
			Objective: rc.Objective,
		})
	}
	return out, nil
}

// parseInsights convierte el JSON de insights (nivel campaña) en entidades.
func parseInsights(body []byte) ([]domain.Insight, error) {
	var env dataEnvelope[rawInsight]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	out := make([]domain.Insight, 0, len(env.Data))
	for _, ri := range env.Data {
		out = append(out, domain.Insight{
			CampaignID:   ri.CampaignID,
			CampaignName: ri.CampaignName,
			Range: domain.DateRange{
				Since: parseDate(ri.DateStart),
				Until: parseDate(ri.DateStop),
			},
			Metrics: metricsFrom(ri),
		})
	}
	return out, nil
}

// parseBreakdown convierte el JSON de insights segmentados en segmentos del
// dominio, tomando la etiqueta del campo correspondiente a la dimensión.
func parseBreakdown(body []byte, dim domain.BreakdownDimension) ([]domain.AudienceSegment, error) {
	var env dataEnvelope[rawInsight]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	out := make([]domain.AudienceSegment, 0, len(env.Data))
	for _, ri := range env.Data {
		out = append(out, domain.AudienceSegment{
			Label:   segmentLabel(ri, dim),
			Metrics: metricsFrom(ri),
		})
	}
	return out, nil
}

// metricsFrom mapea una fila de insight a las métricas del dominio. Las métricas
// de conversión sólo se setean si Meta devolvió el dato (nil = no calculable,
// Principio IX): nunca se fuerza un 0.
func metricsFrom(ri rawInsight) domain.Metrics {
	m := domain.Metrics{
		Spend:       atof(ri.Spend),
		Impressions: atoi(ri.Impressions),
		Clicks:      atoi(ri.Clicks),
		Reach:       atoi(ri.Reach),
		CTR:         atof(ri.CTR),
		CPC:         atof(ri.CPC),
		LinkClicks:  atoi(ri.InlineLinkClicks),
		LinkCTR:     atof(ri.InlineLinkClickCTR),
		Frequency:   atof(ri.Frequency),
	}

	if v, ok := pickAction(ri.Actions); ok {
		n := int64(v)
		m.Purchases = &n
	}
	if v, ok := pickAction(ri.ActionValues); ok {
		m.Revenue = &v
	}
	if v, ok := pickAction(ri.PurchaseROAS); ok {
		m.ROAS = &v
	}
	if v, ok := pickAction(ri.CostPerActionType); ok {
		m.CPA = &v
	}
	return m
}

// pickAction devuelve el valor de compra de un array de actionValue, siguiendo
// la prioridad de purchaseActionTypes. ok=false si no hay ninguna compra.
func pickAction(list []actionValue) (float64, bool) {
	for _, want := range purchaseActionTypes {
		for _, av := range list {
			if av.ActionType == want {
				return atof(av.Value), true
			}
		}
	}
	return 0, false
}

func segmentLabel(ri rawInsight, dim domain.BreakdownDimension) string {
	switch dim {
	case domain.DimensionAge:
		return ri.Age
	case domain.DimensionGender:
		return ri.Gender
	case domain.DimensionRegion:
		return ri.Region
	case domain.DimensionPublisherPlatform:
		return ri.PublisherPlatform
	case domain.DimensionPlatformPosition:
		return ri.PlatformPosition
	default:
		return ""
	}
}

func atof(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func atoi(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(graphDateLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
