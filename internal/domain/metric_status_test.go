package domain

import (
	"testing"
	"time"
)

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if th.MinROAS != 2.0 {
		t.Errorf("MinROAS = %v, want 2.0", th.MinROAS)
	}
	if th.LinkCTRMin != 0.8 || th.LinkCTRMax != 1.5 {
		t.Errorf("LinkCTR range = [%v, %v], want [0.8, 1.5]", th.LinkCTRMin, th.LinkCTRMax)
	}
	if th.MaxFrequency != 3.5 {
		t.Errorf("MaxFrequency = %v, want 3.5", th.MaxFrequency)
	}
	if th.AvgTicket != 80000 {
		t.Errorf("AvgTicket = %v, want 80000", th.AvgTicket)
	}
}

func TestDefaultSufficiency(t *testing.T) {
	s := DefaultSufficiency()
	if s.MinPurchases != 1 || s.MinDays != 7 || s.MinImpressions != 1000 || s.MinLinkClicks != 50 {
		t.Errorf("default sufficiency inesperado: %+v", s)
	}
}

func TestDateRange_Days(t *testing.T) {
	tests := []struct {
		name string
		r    DateRange
		want int
	}{
		{
			"mismo día = 1",
			DateRange{
				Since: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			},
			1,
		},
		{
			"ventana de 30 días",
			DateRange{
				Since: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
			},
			31,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.Days(); got != tc.want {
				t.Errorf("Days() = %d, want %d", got, tc.want)
			}
		})
	}
}
