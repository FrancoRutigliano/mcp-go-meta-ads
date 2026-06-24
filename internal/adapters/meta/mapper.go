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
}

const graphDateLayout = "2006-01-02"

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

// parseInsights convierte el JSON de insights en entidades del dominio. Los
// números de Meta vienen como strings (y a veces ausentes cuando son cero).
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
			Metrics: domain.Metrics{
				Spend:       atof(ri.Spend),
				Impressions: atoi(ri.Impressions),
				Clicks:      atoi(ri.Clicks),
				Reach:       atoi(ri.Reach),
				CTR:         atof(ri.CTR),
				CPC:         atof(ri.CPC),
			},
		})
	}
	return out, nil
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
