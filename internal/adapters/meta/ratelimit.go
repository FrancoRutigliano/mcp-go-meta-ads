package meta

import (
	"context"
	"sync"
	"time"
)

// Limiter espacia las llamadas salientes a Meta para respetar sus límites de
// uso sin "hammering" (Constitución, Principio VI). Es un gate simple que
// garantiza un intervalo mínimo entre solicitudes.
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time // momento más temprano permitido para la próxima llamada
}

// NewLimiter crea un limiter con el intervalo mínimo dado. interval<=0 lo
// desactiva (útil en tests).
func NewLimiter(interval time.Duration) *Limiter {
	return &Limiter{interval: interval}
}

// Wait bloquea hasta que sea seguro emitir la próxima solicitud, o hasta que el
// contexto se cancele.
func (l *Limiter) Wait(ctx context.Context) error {
	if l == nil || l.interval <= 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	var wait time.Duration
	if now.Before(l.next) {
		wait = l.next.Sub(now)
	}
	// Reserva el próximo turno de forma atómica para que llamadas concurrentes
	// se serialicen correctamente.
	start := now
	if wait > 0 {
		start = l.next
	}
	l.next = start.Add(l.interval)
	l.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
