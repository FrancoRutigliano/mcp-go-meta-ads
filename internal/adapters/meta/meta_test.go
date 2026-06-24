package meta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

const testToken = "super-secret-token-xyz"

// newTestClient apunta el cliente a un httptest server, sin rate limiting ni
// esperas reales de backoff.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(testToken, "act_331498724", "v21.0",
		WithBaseURL(srv.URL),
		WithLimiter(NewLimiter(0)),
		WithRetry(2, time.Millisecond),
	)
}

func TestListCampaigns_HappyPath(t *testing.T) {
	var gotPath, gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"id":"6960821349863","name":"Campaña Ventas","status":"ACTIVE","objective":"OUTCOME_SALES"},
			{"id":"6614037282863","name":"Mensajes IG","status":"ACTIVE","objective":"MESSAGES"}
		]}`))
	})

	out, err := client.ListCampaigns(context.Background(), domain.CampaignQuery{OnlyActive: true, Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d campaigns, want 2", len(out))
	}
	if out[0].ID != "6960821349863" || out[0].Status != domain.CampaignActive {
		t.Errorf("unexpected first campaign: %+v", out[0])
	}
	if !strings.Contains(gotPath, "v21.0/act_331498724/campaigns") {
		t.Errorf("path = %q, want account campaigns endpoint", gotPath)
	}
	if !strings.Contains(gotQuery, "effective_status") {
		t.Errorf("query should filter active: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "fields=") {
		t.Errorf("query should request fields: %q", gotQuery)
	}
}

func TestGetInsights_HappyPathParsesStringNumbers(t *testing.T) {
	var gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"campaign_id":"6960821349863","campaign_name":"Campaña Ventas",
			 "spend":"12345.67","impressions":"100000","clicks":"2500","reach":"80000",
			 "ctr":"2.5","cpc":"4.94","date_start":"2026-05-01","date_stop":"2026-05-31"}
		]}`))
	})

	rng := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	out, err := client.GetInsights(context.Background(), domain.InsightQuery{Range: rng})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d insights, want 1", len(out))
	}
	m := out[0].Metrics
	if m.Spend != 12345.67 || m.Impressions != 100000 || m.Clicks != 2500 || m.Reach != 80000 {
		t.Errorf("metrics not parsed correctly: %+v", m)
	}
	if !strings.Contains(gotQuery, "time_range") || !strings.Contains(gotQuery, "level=campaign") {
		t.Errorf("query should include time_range and level=campaign: %q", gotQuery)
	}
}

func TestGetInsights_TargetsCampaignNodeWhenIDProvided(t *testing.T) {
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"data":[]}`))
	})

	rng := domain.DateRange{
		Since: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
	}
	_, err := client.GetInsights(context.Background(), domain.InsightQuery{CampaignID: "6960821349863", Range: rng})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "/6960821349863/insights") {
		t.Errorf("path = %q, want campaign-level insights", gotPath)
	}
}

func TestListCampaigns_InvalidTokenMapsToUnauthorized(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Invalid OAuth access token.","type":"OAuthException","code":190,"fbtrace_id":"abc"}}`))
	})

	_, err := client.ListCampaigns(context.Background(), domain.CampaignQuery{})
	if domain.KindOf(err) != domain.KindUnauthorized {
		t.Errorf("expected unauthorized, got kind=%q err=%v", domain.KindOf(err), err)
	}
}

func TestListCampaigns_RetriesOnRateLimitThenSucceeds(t *testing.T) {
	var hits int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"rate limit","type":"OAuthException","code":17}}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"1","name":"ok","status":"ACTIVE","objective":"MESSAGES"}]}`))
	})

	out, err := client.ListCampaigns(context.Background(), domain.CampaignQuery{})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if hits != 2 {
		t.Errorf("expected 2 hits (1 rate-limited + 1 ok), got %d", hits)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 campaign after retry, got %d", len(out))
	}
}

func TestListCampaigns_GivesUpAfterMaxRetries(t *testing.T) {
	var hits int
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limit","code":17}}`))
	})

	_, err := client.ListCampaigns(context.Background(), domain.CampaignQuery{})
	if domain.KindOf(err) != domain.KindRateLimited {
		t.Errorf("expected rate_limited after exhausting retries, got %v", err)
	}
	if hits != 3 { // 1 inicial + 2 reintentos
		t.Errorf("expected 3 attempts, got %d", hits)
	}
}

// Constitución, Principio I/V: ni el token ni detalles crudos deben filtrarse en
// el error que sale del adaptador.
func TestErrors_NeverLeakToken(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"boom","code":190}}`))
	})

	_, err := client.ListCampaigns(context.Background(), domain.CampaignQuery{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), testToken) {
		t.Fatalf("error leaked token: %v", err)
	}
}

func TestLimiter_EnforcesMinInterval(t *testing.T) {
	l := NewLimiter(30 * time.Millisecond)
	ctx := context.Background()

	start := time.Now()
	_ = l.Wait(ctx) // primera no espera
	_ = l.Wait(ctx) // segunda debe esperar ~30ms
	elapsed := time.Since(start)

	if elapsed < 25*time.Millisecond {
		t.Errorf("limiter did not space calls: elapsed=%v", elapsed)
	}
}
