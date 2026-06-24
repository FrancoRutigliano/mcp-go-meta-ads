package domain

import (
	"errors"
	"testing"
	"time"
)

func TestKindOf_FindsKindThroughWrapping(t *testing.T) {
	base := NewError(KindRateLimited, "meta.ListCampaigns", errors.New("429 too many requests"))
	wrapped := errors.New("contexto extra: " + base.Error())
	_ = wrapped // un error plano no envuelve; verificamos el caso directo y el envuelto real

	if got := KindOf(base); got != KindRateLimited {
		t.Errorf("KindOf(base) = %q, want %q", got, KindRateLimited)
	}

	// Envuelto con %w debe seguir siendo detectable.
	chained := wrap(base)
	if got := KindOf(chained); got != KindRateLimited {
		t.Errorf("KindOf(chained) = %q, want %q", got, KindRateLimited)
	}
}

func wrap(err error) error {
	return &wrapErr{err}
}

type wrapErr struct{ inner error }

func (w *wrapErr) Error() string { return "wrap: " + w.inner.Error() }
func (w *wrapErr) Unwrap() error { return w.inner }

func TestKindOf_UnknownForPlainError(t *testing.T) {
	if got := KindOf(errors.New("boom")); got != KindUnknown {
		t.Errorf("KindOf(plain) = %q, want %q", got, KindUnknown)
	}
	if got := KindOf(nil); got != KindUnknown {
		t.Errorf("KindOf(nil) = %q, want %q", got, KindUnknown)
	}
}

func TestError_UnwrapReturnsCause(t *testing.T) {
	cause := errors.New("causa raíz")
	err := NewError(KindUpstream, "op", cause)
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is should find the cause")
	}
}

func TestDateRange_Valid(t *testing.T) {
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		r       DateRange
		wantErr bool
	}{
		{"rango válido", DateRange{Since: since, Until: until}, false},
		{"mismo día", DateRange{Since: since, Until: since}, false},
		{"invertido", DateRange{Since: until, Until: since}, true},
		{"since cero", DateRange{Until: until}, true},
		{"until cero", DateRange{Since: since}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Valid()
			if (err != nil) != tc.wantErr {
				t.Errorf("Valid() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
