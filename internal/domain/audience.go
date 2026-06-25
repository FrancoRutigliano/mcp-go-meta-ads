package domain

import "slices"

// BreakdownDimension es la dimensión por la que se segmenta el rendimiento.
// Los valores coinciden con los nombres de breakdown de la plataforma.
type BreakdownDimension string

const (
	DimensionAge               BreakdownDimension = "age"
	DimensionGender            BreakdownDimension = "gender"
	DimensionRegion            BreakdownDimension = "region"
	DimensionPublisherPlatform BreakdownDimension = "publisher_platform"
	DimensionPlatformPosition  BreakdownDimension = "platform_position"
)

// ValidDimensions enumera las dimensiones soportadas por esta feature.
func ValidDimensions() []BreakdownDimension {
	return []BreakdownDimension{
		DimensionAge,
		DimensionGender,
		DimensionRegion,
		DimensionPublisherPlatform,
		DimensionPlatformPosition,
	}
}

// Valid indica si la dimensión está soportada.
func (d BreakdownDimension) Valid() bool {
	return slices.Contains(ValidDimensions(), d)
}

// AudienceSegment es un segmento (p. ej. "25-34" para edad) con sus métricas.
type AudienceSegment struct {
	Label   string // valor del segmento devuelto por la plataforma
	Metrics Metrics
}

// AudienceBreakdown es el rendimiento segmentado por una dimensión.
type AudienceBreakdown struct {
	Dimension BreakdownDimension
	Range     DateRange
	Segments  []AudienceSegment
}

// AudienceQuery parametriza el desglose. CampaignID vacío opera a nivel cuenta.
type AudienceQuery struct {
	CampaignID string
	Dimension  BreakdownDimension
	Range      DateRange
}
