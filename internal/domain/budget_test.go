package domain

import (
	"errors"
	"testing"
)

func TestParseMoneyMinorUnits(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantCents int64
		wantErr   bool
	}{
		{name: "monto típico en centavos", raw: "150000", wantCents: 150000},
		{name: "cero explícito", raw: "0", wantCents: 0},
		{name: "con espacios alrededor", raw: " 250000 ", wantCents: 250000},
		{name: "vacío es error", raw: "", wantErr: true},
		{name: "no numérico es error", raw: "mucha plata", wantErr: true},
		{name: "negativo es error", raw: "-100", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMoneyMinorUnits(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("esperaba error para %q, no hubo", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got.Cents != tt.wantCents {
				t.Errorf("Cents = %d, esperaba %d", got.Cents, tt.wantCents)
			}
		})
	}
}

func TestMoneyConversions(t *testing.T) {
	m := MoneyFromPesos(1500)
	if m.Cents != 150000 {
		t.Errorf("MoneyFromPesos(1500).Cents = %d, esperaba 150000", m.Cents)
	}
	if got := m.Pesos(); got != 1500 {
		t.Errorf("Pesos() = %v, esperaba 1500", got)
	}
	// MinorUnits es lo que se manda a Meta: entero, sin separadores ni decimales.
	if got := m.MinorUnits(); got != "150000" {
		t.Errorf("MinorUnits() = %q, esperaba \"150000\"", got)
	}
	if !MoneyFromPesos(0).IsZero() {
		t.Error("MoneyFromPesos(0) debería ser IsZero")
	}
}

func TestBudgetTypeValid(t *testing.T) {
	if !BudgetDaily.Valid() || !BudgetLifetime.Valid() {
		t.Error("los tipos daily y lifetime deberían ser válidos")
	}
	if BudgetType("semanal").Valid() {
		t.Error("un tipo desconocido no debería ser válido")
	}
}

func TestBudgetLevelValid(t *testing.T) {
	if !LevelCampaign.Valid() || !LevelAdSet.Valid() {
		t.Error("los niveles campaign y adset deberían ser válidos")
	}
	if BudgetLevel("cuenta").Valid() {
		t.Error("un nivel desconocido no debería ser válido")
	}
}

func TestBudgetChangeValidate(t *testing.T) {
	amount := MoneyFromPesos(5000)
	pct := 30.0

	tests := []struct {
		name    string
		change  BudgetChange
		wantErr error
	}{
		{
			name:   "sólo monto absoluto es válido",
			change: BudgetChange{Amount: &amount},
		},
		{
			name:   "sólo porcentaje es válido",
			change: BudgetChange{Percent: &pct},
		},
		{
			name:    "ninguno de los dos es inválido",
			change:  BudgetChange{},
			wantErr: ErrNoBudgetChange,
		},
		{
			name:    "ambos a la vez es inválido",
			change:  BudgetChange{Amount: &amount, Percent: &pct},
			wantErr: ErrBothAmountAndPercent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.change.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("error inesperado: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, esperaba que envolviera %v", err, tt.wantErr)
			}
		})
	}
}

func TestBudgetChangeValidateRejectsBadAmounts(t *testing.T) {
	zero := MoneyFromPesos(0)
	negative := Money{Cents: -100}
	tooBigPct := 100000.0

	tests := []struct {
		name   string
		change BudgetChange
	}{
		{name: "monto cero", change: BudgetChange{Amount: &zero}},
		{name: "monto negativo", change: BudgetChange{Amount: &negative}},
		{name: "porcentaje que anula el presupuesto", change: BudgetChange{Percent: pctPtr(-100)}},
		{name: "porcentaje por debajo de -100", change: BudgetChange{Percent: pctPtr(-150)}},
		{name: "porcentaje absurdo", change: BudgetChange{Percent: &tooBigPct}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.change.Validate(); err == nil {
				t.Error("esperaba error, no hubo")
			}
		})
	}
}

func TestBudgetChangeApply(t *testing.T) {
	current := MoneyFromPesos(1500)

	tests := []struct {
		name      string
		change    BudgetChange
		wantPesos float64
	}{
		{
			name:      "monto absoluto se usa tal cual",
			change:    BudgetChange{Amount: moneyPtr(MoneyFromPesos(5000))},
			wantPesos: 5000,
		},
		{
			name:      "sube 30 por ciento",
			change:    BudgetChange{Percent: pctPtr(30)},
			wantPesos: 1950,
		},
		{
			name:      "baja a la mitad",
			change:    BudgetChange{Percent: pctPtr(-50)},
			wantPesos: 750,
		},
		{
			// 1500 * 1.333... = 1999,5 → redondeo a peso entero.
			name:      "redondea a peso entero",
			change:    BudgetChange{Percent: pctPtr(33.3)},
			wantPesos: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.change.Apply(current)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got.Pesos() != tt.wantPesos {
				t.Errorf("Apply() = %v pesos, esperaba %v", got.Pesos(), tt.wantPesos)
			}
			// El resultado siempre queda en pesos enteros: lo que se aprueba es
			// exactamente lo que se aplica (SC-009).
			if got.Cents%minorUnitsPerPeso != 0 {
				t.Errorf("Apply() = %d centavos, debería ser múltiplo de %d", got.Cents, minorUnitsPerPeso)
			}
		})
	}
}

func TestBudgetChangeApplyRejectsRelativeWithoutBase(t *testing.T) {
	change := BudgetChange{Percent: pctPtr(30)}

	_, err := change.Apply(Money{})
	if !errors.Is(err, ErrNoBudgetToScale) {
		t.Errorf("error = %v, esperaba que envolviera ErrNoBudgetToScale", err)
	}
}

func TestBudgetChangeApplyRejectsUnchanged(t *testing.T) {
	current := MoneyFromPesos(1500)

	_, err := BudgetChange{Amount: moneyPtr(current)}.Apply(current)
	if !errors.Is(err, ErrBudgetUnchanged) {
		t.Errorf("error = %v, esperaba que envolviera ErrBudgetUnchanged", err)
	}
}

func moneyPtr(m Money) *Money   { return &m }
func pctPtr(p float64) *float64 { return &p }
