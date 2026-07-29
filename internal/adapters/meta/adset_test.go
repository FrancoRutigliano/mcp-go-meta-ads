package meta

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestGetAdSets_ParsesBudgets(t *testing.T) {
	var gotPath, gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"id":"as1","name":"Público frío","status":"ACTIVE","campaign_id":"c1","daily_budget":"150000"},
			{"id":"as2","name":"Remarketing","status":"PAUSED","campaign_id":"c1","daily_budget":"80000"}
		]}`))
	})

	sets, err := client.GetAdSets(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sets) != 2 {
		t.Fatalf("esperaba 2 conjuntos, vinieron %d", len(sets))
	}
	if sets[0].Name != "Público frío" || sets[0].Budget.Amount.Pesos() != 1500 {
		t.Errorf("conjunto mal parseado: %+v", sets[0])
	}
	if sets[1].Status != domain.CampaignPaused {
		t.Errorf("estado = %q, esperaba PAUSED", sets[1].Status)
	}
	if !strings.Contains(gotPath, "/c1/adsets") {
		t.Errorf("path = %q, esperaba el endpoint de conjuntos de la campaña", gotPath)
	}
	if !strings.Contains(gotQuery, "daily_budget") {
		t.Errorf("la consulta debe pedir los campos de presupuesto: %q", gotQuery)
	}
}

func TestGetAdSets_EmptyIsNotAnError(t *testing.T) {
	// Una campaña sin conjuntos devuelve lista vacía; quién decide que eso es un
	// problema es el caso de uso, no el adaptador.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	})

	sets, err := client.GetAdSets(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sets) != 0 {
		t.Errorf("esperaba lista vacía, vinieron %d", len(sets))
	}
}

func TestGetAdSets_RequiresCampaignID(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llegar a Meta")
	})

	_, err := client.GetAdSets(context.Background(), "")
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("Kind = %v, esperaba invalid_input", domain.KindOf(err))
	}
}

func TestGetAdSet_ParsesSingleObject(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"as1","name":"Público frío","status":"ACTIVE","campaign_id":"c1","daily_budget":"150000"}`))
	})

	set, err := client.GetAdSet(context.Background(), "as1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.ID != "as1" || set.CampaignID != "c1" {
		t.Errorf("conjunto mal parseado: %+v", set)
	}
	if !set.ManagesOwnBudget() || set.Budget.Amount.Pesos() != 1500 {
		t.Errorf("presupuesto mal parseado: %+v", set.Budget)
	}
}

func TestGetAdSet_NoBudgetMeansCampaignLevel(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"as1","name":"Público frío","status":"ACTIVE","campaign_id":"c1","daily_budget":"0"}`))
	})

	set, err := client.GetAdSet(context.Background(), "as1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.ManagesOwnBudget() {
		t.Error("sin presupuesto propio, el conjunto no lo administra")
	}
}

func TestUpdateAdSetBudget_PostsToAdSetNode(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		w.Write([]byte(`{"success":true}`))
	})

	err := client.UpdateAdSetBudget(context.Background(), "as1", domain.Budget{
		Type:   domain.BudgetDaily,
		Amount: domain.MoneyFromPesos(2500),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("método = %q, esperaba POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/as1") {
		t.Errorf("path = %q, esperaba el nodo del conjunto", gotPath)
	}
	if !strings.Contains(gotQuery, "daily_budget=250000") {
		t.Errorf("query debe incluir daily_budget=250000: %q", gotQuery)
	}
}

func TestUpdateAdSetBudget_RequiresID(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llegar a Meta")
	})

	err := client.UpdateAdSetBudget(context.Background(), "", domain.Budget{
		Type:   domain.BudgetDaily,
		Amount: domain.MoneyFromPesos(2500),
	})
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("Kind = %v, esperaba invalid_input", domain.KindOf(err))
	}
}
