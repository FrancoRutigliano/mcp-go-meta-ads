package memstore

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

func TestStore_SaveGetDelete(t *testing.T) {
	s := New()

	if _, ok := s.Get("nope"); ok {
		t.Error("store vacío no debe encontrar nada")
	}

	p := domain.Proposal{ID: "prop_1", CampaignID: "c1"}
	s.Save(p)

	got, ok := s.Get("prop_1")
	if !ok || got.CampaignID != "c1" {
		t.Errorf("Get tras Save falló: %+v ok=%v", got, ok)
	}

	s.Delete("prop_1")
	if _, ok := s.Get("prop_1"); ok {
		t.Error("Delete no eliminó la propuesta")
	}
}

func TestStore_ConcurrentProposals(t *testing.T) {
	// Reasignar plata entre campañas genera varias propuestas en vuelo; el store
	// debe soportarlas en paralelo sin corromperse (correr con -race).
	store := New()
	const n = 50

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("prop_%d", i)
			store.Save(domain.Proposal{ID: id, Kind: domain.ProposalBudget, ExpiresAt: time.Now().Add(time.Minute)})
			if _, ok := store.Get(id); !ok {
				t.Errorf("no encontré la propuesta %s recién guardada", id)
			}
			store.Delete(id)
		}(i)
	}
	wg.Wait()

	for i := range n {
		if _, ok := store.Get(fmt.Sprintf("prop_%d", i)); ok {
			t.Fatalf("la propuesta %d debería estar consumida", i)
		}
	}
}
