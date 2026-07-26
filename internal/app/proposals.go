package app

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

// ProposalStore guarda las propuestas pendientes entre el paso propose y el
// confirm (Constitución, Principio II). El confirm sólo puede ejecutar una
// propuesta que exista acá; por eso Claude no puede saltear el propose.
type ProposalStore interface {
	Save(p domain.Proposal)
	Get(id string) (domain.Proposal, bool)
	Delete(id string)
}

// newProposalID genera un identificador aleatorio no adivinable, para que una
// propuesta sólo pueda confirmarse si realmente existió su propose.
func newProposalID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "prop_" + hex.EncodeToString(b)
}
