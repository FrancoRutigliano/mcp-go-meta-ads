package meta

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestGetCampaign_ParsesDailyBudget(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"c1","name":"Ventas","status":"ACTIVE","objective":"OUTCOME_SALES","daily_budget":"300000"}`))
	})

	c, err := client.GetCampaign(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.ManagesOwnBudget() {
		t.Fatal("con daily_budget, la campaña administra su propio presupuesto")
	}
	if c.Budget.Type != domain.BudgetDaily {
		t.Errorf("Type = %q, esperaba daily", c.Budget.Type)
	}
	// 300000 unidades menores = $3.000 ARS.
	if c.Budget.Amount.Pesos() != 3000 {
		t.Errorf("monto = %v pesos, esperaba 3000", c.Budget.Amount.Pesos())
	}
}

func TestGetCampaign_ParsesLifetimeBudget(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"c1","name":"Verano","status":"ACTIVE","lifetime_budget":"5000000"}`))
	})

	c, err := client.GetCampaign(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Budget == nil || c.Budget.Type != domain.BudgetLifetime {
		t.Fatalf("esperaba presupuesto total, vino %+v", c.Budget)
	}
	if c.Budget.Amount.Pesos() != 50000 {
		t.Errorf("monto = %v pesos, esperaba 50000", c.Budget.Amount.Pesos())
	}
}

func TestGetCampaign_NoBudgetMeansAdSetLevel(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "campo ausente",
			body: `{"id":"c1","name":"Ventas","status":"ACTIVE"}`,
		},
		{
			// Meta devuelve "0" cuando el presupuesto se administra en los conjuntos.
			name: "campo en cero",
			body: `{"id":"c1","name":"Ventas","status":"ACTIVE","daily_budget":"0","lifetime_budget":"0"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tt.body))
			})

			c, err := client.GetCampaign(context.Background(), "c1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.ManagesOwnBudget() {
				t.Errorf("sin presupuesto propio, ManagesOwnBudget debe ser false (Budget=%+v)", c.Budget)
			}
		})
	}
}

func TestListCampaigns_ParsesBudgets(t *testing.T) {
	var gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"id":"c1","name":"Ventas","status":"ACTIVE","daily_budget":"300000"},
			{"id":"c2","name":"Remarketing","status":"PAUSED"}
		]}`))
	})

	cs, err := client.ListCampaigns(context.Background(), domain.CampaignQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("esperaba 2 campañas, vinieron %d", len(cs))
	}
	if !cs[0].ManagesOwnBudget() || cs[1].ManagesOwnBudget() {
		t.Errorf("presupuestos mal resueltos: %+v / %+v", cs[0].Budget, cs[1].Budget)
	}
	// Los campos nuevos deben pedirse explícitamente a la Graph API.
	if !strings.Contains(gotQuery, "daily_budget") || !strings.Contains(gotQuery, "lifetime_budget") {
		t.Errorf("la consulta debe pedir los campos de presupuesto: %q", gotQuery)
	}
}

func TestGetCampaign_InvalidBudgetIsUpstreamError(t *testing.T) {
	// Un monto que no se puede interpretar no se convierte en cero silencioso
	// (Principio V): se propaga como error.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"c1","name":"Ventas","status":"ACTIVE","daily_budget":"mucha plata"}`))
	})

	_, err := client.GetCampaign(context.Background(), "c1")
	if domain.KindOf(err) != domain.KindUpstream {
		t.Errorf("Kind = %v, esperaba upstream (err: %v)", domain.KindOf(err), err)
	}
}

func TestUpdateCampaignBudget_PostsDailyBudgetInMinorUnits(t *testing.T) {
	var gotMethod, gotQuery, gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery, gotPath = r.Method, r.URL.RawQuery, r.URL.Path
		w.Write([]byte(`{"success":true}`))
	})

	err := client.UpdateCampaignBudget(context.Background(), "c1", domain.Budget{
		Type:   domain.BudgetDaily,
		Amount: domain.MoneyFromPesos(5000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("método = %q, esperaba POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/c1") {
		t.Errorf("path = %q, esperaba el nodo de la campaña", gotPath)
	}
	// $5.000 ARS = 500000 unidades menores.
	if !strings.Contains(gotQuery, "daily_budget=500000") {
		t.Errorf("query debe incluir daily_budget=500000: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "lifetime_budget") {
		t.Errorf("no debe mandar el otro tipo de presupuesto: %q", gotQuery)
	}
}

func TestUpdateCampaignBudget_PostsLifetimeBudget(t *testing.T) {
	var gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"success":true}`))
	})

	err := client.UpdateCampaignBudget(context.Background(), "c1", domain.Budget{
		Type:   domain.BudgetLifetime,
		Amount: domain.MoneyFromPesos(50000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "lifetime_budget=5000000") {
		t.Errorf("query debe incluir lifetime_budget=5000000: %q", gotQuery)
	}
}

func TestUpdateCampaignBudget_Rejections(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llegar a Meta")
	})

	tests := []struct {
		name   string
		id     string
		budget domain.Budget
	}{
		{name: "sin id", id: "", budget: domain.Budget{Type: domain.BudgetDaily, Amount: domain.MoneyFromPesos(5000)}},
		{name: "tipo inválido", id: "c1", budget: domain.Budget{Type: "semanal", Amount: domain.MoneyFromPesos(5000)}},
		{name: "monto cero", id: "c1", budget: domain.Budget{Type: domain.BudgetDaily}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.UpdateCampaignBudget(context.Background(), tt.id, tt.budget)
			if domain.KindOf(err) != domain.KindInvalidInput {
				t.Errorf("Kind = %v, esperaba invalid_input", domain.KindOf(err))
			}
		})
	}
}

func TestUpdateCampaignBudget_PermissionErrorMapsToUnauthorized(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"(#200) Permissions error","type":"OAuthException","code":200}}`))
	})

	err := client.UpdateCampaignBudget(context.Background(), "c1", domain.Budget{
		Type:   domain.BudgetDaily,
		Amount: domain.MoneyFromPesos(5000),
	})
	if domain.KindOf(err) != domain.KindUnauthorized {
		t.Errorf("Kind = %v, esperaba unauthorized", domain.KindOf(err))
	}
	// El token nunca debe aparecer en el error que se propaga.
	if strings.Contains(err.Error(), testToken) {
		t.Error("el error no debe contener el token")
	}
}

func TestUpdateCampaignBudget_BelowMinimumIsRecognized(t *testing.T) {
	// R5 del research: no tenemos confirmado el subcódigo de Meta, así que se
	// detecta por el texto. Este test fija el comportamiento del fallback.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Daily budget must be at least $1.00","type":"OAuthException","code":100}}`))
	})

	err := client.UpdateCampaignBudget(context.Background(), "c1", domain.Budget{
		Type:   domain.BudgetDaily,
		Amount: domain.MoneyFromPesos(1),
	})

	if !errors.Is(err, domain.ErrBudgetBelowMinimum) {
		t.Errorf("error = %v, esperaba que envolviera ErrBudgetBelowMinimum", err)
	}
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("Kind = %v, esperaba invalid_input", domain.KindOf(err))
	}
}

func TestTranslateError_UnrelatedBudgetMessageIsNotMisread(t *testing.T) {
	// Un error que menciona "budget" pero no es de mínimo no debe capturarse.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"The campaign budget cannot be edited while the campaign is in review","type":"OAuthException","code":100}}`))
	})

	err := client.UpdateCampaignBudget(context.Background(), "c1", domain.Budget{
		Type:   domain.BudgetDaily,
		Amount: domain.MoneyFromPesos(5000),
	})

	if errors.Is(err, domain.ErrBudgetBelowMinimum) {
		t.Errorf("no debería interpretarse como presupuesto mínimo: %v", err)
	}
}
