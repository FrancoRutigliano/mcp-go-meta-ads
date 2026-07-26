package domain

// AdInsight es el rendimiento de un anuncio (creativo) individual, con su
// contexto de campaña. Es lo que la usuaria piensa como "la publicación/anuncio".
type AdInsight struct {
	AdID         string
	AdName       string
	CampaignID   string
	CampaignName string
	Range        DateRange
	Metrics      Metrics
}

// AdQuery parametriza la lectura de rendimiento por anuncio. CampaignID vacío
// opera a nivel de toda la cuenta.
type AdQuery struct {
	CampaignID string
	Range      DateRange
	Limit      int
}
