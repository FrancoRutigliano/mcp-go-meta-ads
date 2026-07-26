package meta

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestGetCampaign_ParsesSingleObject(t *testing.T) {
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"id":"6960821349863","name":"Ventas","status":"ACTIVE","objective":"OUTCOME_SALES"}`))
	})

	c, err := client.GetCampaign(context.Background(), "6960821349863")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "Ventas" || c.Status != domain.CampaignActive {
		t.Errorf("campaña mal parseada: %+v", c)
	}
	if !strings.Contains(gotPath, "/6960821349863") {
		t.Errorf("path = %q", gotPath)
	}
}

func TestUpdateCampaignStatus_PostsStatus(t *testing.T) {
	var gotMethod, gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"success":true}`))
	})

	err := client.UpdateCampaignStatus(context.Background(), "6960821349863", domain.CampaignPaused)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("método = %q, want POST", gotMethod)
	}
	if !strings.Contains(gotQuery, "status=PAUSED") {
		t.Errorf("query debe incluir status=PAUSED: %q", gotQuery)
	}
}

func TestUpdateCampaignStatus_ReadOnlyTokenMapsToUnauthorized(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"(#200) Permissions error","type":"OAuthException","code":200}}`))
	})

	err := client.UpdateCampaignStatus(context.Background(), "1", domain.CampaignPaused)
	if domain.KindOf(err) != domain.KindUnauthorized {
		t.Errorf("permiso denegado debe ser unauthorized, got kind=%q err=%v", domain.KindOf(err), err)
	}
}
