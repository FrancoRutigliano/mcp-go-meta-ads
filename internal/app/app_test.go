package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

// fakeReader es un doble de ports.MetaReader que registra la query recibida y
// devuelve datos/errores predefinidos.
type fakeReader struct {
	campaigns    []domain.Campaign
	insights     []domain.Insight
	breakdown    domain.AudienceBreakdown
	err          error
	gotCampaignQ domain.CampaignQuery
	gotInsightQ  domain.InsightQuery
	gotAudienceQ domain.AudienceQuery
	called       bool
}

func (f *fakeReader) ListCampaigns(_ context.Context, q domain.CampaignQuery) ([]domain.Campaign, error) {
	f.called = true
	f.gotCampaignQ = q
	return f.campaigns, f.err
}

func (f *fakeReader) GetInsights(_ context.Context, q domain.InsightQuery) ([]domain.Insight, error) {
	f.called = true
	f.gotInsightQ = q
	return f.insights, f.err
}

func (f *fakeReader) GetAudienceBreakdown(_ context.Context, q domain.AudienceQuery) (domain.AudienceBreakdown, error) {
	f.called = true
	f.gotAudienceQ = q
	return f.breakdown, f.err
}

// helpers de umbrales/suficiencia por defecto para construir casos de uso.
func defThresholds() domain.Thresholds         { return domain.DefaultThresholds() }
func defSufficiency() domain.SufficiencyPolicy { return domain.DefaultSufficiency() }

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestListCampaigns_AppliesDefaultLimit(t *testing.T) {
	fake := &fakeReader{}
	uc := NewListCampaigns(fake)

	_, err := uc.Execute(context.Background(), true, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotCampaignQ.Limit != DefaultCampaignLimit {
		t.Errorf("Limit = %d, want default %d", fake.gotCampaignQ.Limit, DefaultCampaignLimit)
	}
	if !fake.gotCampaignQ.OnlyActive {
		t.Errorf("OnlyActive should pass through as true")
	}
}

func TestListCampaigns_RespectsExplicitLimit(t *testing.T) {
	fake := &fakeReader{}
	uc := NewListCampaigns(fake)

	if _, err := uc.Execute(context.Background(), false, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotCampaignQ.Limit != 10 {
		t.Errorf("Limit = %d, want 10", fake.gotCampaignQ.Limit)
	}
}

func TestListCampaigns_PropagatesError(t *testing.T) {
	sentinel := domain.NewError(domain.KindUpstream, "meta.ListCampaigns", errors.New("boom"))
	fake := &fakeReader{err: sentinel}
	uc := NewListCampaigns(fake)

	_, err := uc.Execute(context.Background(), false, 0)
	if domain.KindOf(err) != domain.KindUpstream {
		t.Errorf("expected upstream kind, got %v", err)
	}
}

func TestGetInsights_AppliesDefault30DayRange(t *testing.T) {
	now := time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
	fake := &fakeReader{}
	uc := NewGetInsightsWithClock(fake, defThresholds(), defSufficiency(), fixedClock(now))

	_, applied, err := uc.Execute(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantUntil := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	wantSince := wantUntil.AddDate(0, 0, -defaultInsightDays)
	if !applied.Since.Equal(wantSince) || !applied.Until.Equal(wantUntil) {
		t.Errorf("applied range = [%v, %v], want [%v, %v]",
			applied.Since, applied.Until, wantSince, wantUntil)
	}
	// La query al reader debe llevar el mismo rango por defecto.
	if !fake.gotInsightQ.Range.Since.Equal(wantSince) {
		t.Errorf("reader range.Since = %v, want %v", fake.gotInsightQ.Range.Since, wantSince)
	}
}

func TestGetInsights_RejectsInvalidRangeWithoutCallingReader(t *testing.T) {
	fake := &fakeReader{}
	uc := NewGetInsights(fake, defThresholds(), defSufficiency())

	bad := &domain.DateRange{
		Since: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), // invertido
	}
	_, _, err := uc.Execute(context.Background(), "123", bad)

	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("expected invalid_input, got %v", err)
	}
	if fake.called {
		t.Errorf("reader must NOT be called on invalid input")
	}
}

func TestGetInsights_PassesThroughCampaignAndRange(t *testing.T) {
	fake := &fakeReader{}
	uc := NewGetInsights(fake, defThresholds(), defSufficiency())

	rng := &domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	if _, _, err := uc.Execute(context.Background(), "6960821349863", rng); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotInsightQ.CampaignID != "6960821349863" {
		t.Errorf("campaignID not passed through: %q", fake.gotInsightQ.CampaignID)
	}
	if !fake.gotInsightQ.Range.Since.Equal(rng.Since) {
		t.Errorf("range not passed through")
	}
}
