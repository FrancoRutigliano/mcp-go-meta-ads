package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mashats/meta-ads-manager/internal/adapters/memstore"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

type fakeWriter struct {
	calls     int
	gotID     string
	gotStatus domain.CampaignStatus
	err       error
}

func (w *fakeWriter) UpdateCampaignStatus(_ context.Context, id string, s domain.CampaignStatus) error {
	w.calls++
	w.gotID = id
	w.gotStatus = s
	return w.err
}

func TestProposeCampaignStatus_BuildsProposalWithoutWriting(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{ID: "1", Name: "Ventas", Status: domain.CampaignActive}}
	store := memstore.New()
	uc := NewProposeCampaignStatus(fake, store)

	p, err := uc.Execute(context.Background(), "1", domain.ActionPause)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Before != "ACTIVE" || p.After != "PAUSED" {
		t.Errorf("propuesta mal armada: %+v", p)
	}
	if _, ok := store.Get(p.ID); !ok {
		t.Error("la propuesta debe quedar guardada en el store")
	}
}

func TestProposeCampaignStatus_RejectsNoOp(t *testing.T) {
	fake := &fakeReader{campaign: domain.Campaign{ID: "1", Status: domain.CampaignPaused}}
	uc := NewProposeCampaignStatus(fake, memstore.New())

	_, err := uc.Execute(context.Background(), "1", domain.ActionPause) // ya está pausada
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("pausar algo ya pausado debe ser invalid_input, got %v", err)
	}
}

func TestConfirmProposal_AppliesAndAudits(t *testing.T) {
	store := memstore.New()
	writer := &fakeWriter{}
	var buf bytes.Buffer
	audit := slog.New(slog.NewTextHandler(&buf, nil))
	uc := NewConfirmProposal(store, writer, audit)

	now := time.Now()
	p := domain.Proposal{
		ID: "prop_x", Kind: domain.ProposalCampaignStatus, CampaignID: "1", CampaignName: "Ventas",
		Field: "estado", Before: "ACTIVE", After: "PAUSED", ExpiresAt: now.Add(5 * time.Minute),
	}
	store.Save(p)

	out, err := uc.Execute(context.Background(), "prop_x", "Mariana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer.calls != 1 || writer.gotStatus != domain.CampaignPaused {
		t.Errorf("no aplicó el cambio correcto: %+v", writer)
	}
	if _, ok := store.Get("prop_x"); ok {
		t.Error("la propuesta debe consumirse (un solo uso)")
	}
	if out.CampaignName != "Ventas" {
		t.Errorf("debe devolver la propuesta aplicada")
	}
	log := buf.String()
	for _, want := range []string{"escritura confirmada", "antes=ACTIVE", "despues=PAUSED", "confirmado_por=Mariana"} {
		if !strings.Contains(log, want) {
			t.Errorf("auditoría sin %q en: %s", want, log)
		}
	}
}

// Principio II: no se puede confirmar algo que no fue propuesto.
func TestConfirmProposal_UnknownIDRejected(t *testing.T) {
	uc := NewConfirmProposal(memstore.New(), &fakeWriter{}, slog.Default())
	_, err := uc.Execute(context.Background(), "prop_inventado", "x")
	if domain.KindOf(err) != domain.KindNotFound {
		t.Errorf("id inexistente debe ser not_found, got %v", err)
	}
}

func TestConfirmProposal_ExpiredRejected(t *testing.T) {
	store := memstore.New()
	store.Save(domain.Proposal{ID: "prop_v", Kind: domain.ProposalCampaignStatus, ExpiresAt: time.Now().Add(-time.Minute)})
	writer := &fakeWriter{}
	uc := NewConfirmProposal(store, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_v", "x")
	if domain.KindOf(err) != domain.KindInvalidInput {
		t.Errorf("propuesta vencida debe ser invalid_input, got %v", err)
	}
	if writer.calls != 0 {
		t.Error("no debe escribir si la propuesta venció")
	}
}

func TestConfirmProposal_KeepsProposalOnWriteFailure(t *testing.T) {
	store := memstore.New()
	store.Save(domain.Proposal{
		ID: "prop_e", Kind: domain.ProposalCampaignStatus, CampaignID: "1",
		After: "PAUSED", ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	writer := &fakeWriter{err: domain.NewError(domain.KindUnauthorized, "meta", errors.New("solo lectura"))}
	uc := NewConfirmProposal(store, writer, slog.Default())

	_, err := uc.Execute(context.Background(), "prop_e", "x")
	if domain.KindOf(err) != domain.KindUnauthorized {
		t.Errorf("debe propagar el error de escritura, got %v", err)
	}
	if _, ok := store.Get("prop_e"); !ok {
		t.Error("si falla la escritura, la propuesta NO debe consumirse (para reintentar)")
	}
}
