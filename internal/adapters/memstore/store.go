// Package memstore implementa un almacén de propuestas en memoria.
//
// Es suficiente para una sola instancia (el caso de Railway hoy). Si en el
// futuro se corre en varias instancias, habría que migrar a un store compartido
// (p. ej. Redis), porque una propuesta creada en una instancia no existiría en
// otra.
package memstore

import (
	"sync"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

// Store guarda propuestas por ID, protegido con mutex.
type Store struct {
	mu sync.Mutex
	m  map[string]domain.Proposal
}

// New crea un store vacío.
func New() *Store {
	return &Store{m: make(map[string]domain.Proposal)}
}

// Save guarda (o reemplaza) una propuesta.
func (s *Store) Save(p domain.Proposal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[p.ID] = p
}

// Get devuelve una propuesta por ID.
func (s *Store) Get(id string) (domain.Proposal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.m[id]
	return p, ok
}

// Delete elimina una propuesta (consumo de un solo uso).
func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}
