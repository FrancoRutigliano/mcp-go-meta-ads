package domain

import (
	"testing"
	"time"
)

func TestCampaignAction(t *testing.T) {
	if !ActionPause.Valid() || !ActionActivate.Valid() {
		t.Error("pause/activate deben ser válidas")
	}
	if CampaignAction("delete").Valid() {
		t.Error("delete no debe ser válida")
	}
	if ActionPause.TargetStatus() != CampaignPaused {
		t.Error("pause → PAUSED")
	}
	if ActionActivate.TargetStatus() != CampaignActive {
		t.Error("activate → ACTIVE")
	}
}

func TestProposal_Expired(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	p := Proposal{ExpiresAt: now.Add(5 * time.Minute)}

	if p.Expired(now) {
		t.Error("no debería estar vencida antes de ExpiresAt")
	}
	if !p.Expired(now.Add(6 * time.Minute)) {
		t.Error("debería estar vencida después de ExpiresAt")
	}
}
