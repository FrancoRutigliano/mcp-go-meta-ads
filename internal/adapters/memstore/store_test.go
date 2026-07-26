package memstore

import (
	"testing"

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
