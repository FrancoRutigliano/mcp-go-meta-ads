package domain

import (
	"errors"
	"testing"
)

func testGuardrails() Guardrails {
	return Guardrails{
		MaxIncreaseFactor: 3,
		MaxDailyBudget:    MoneyFromPesos(25000),
	}
}

func TestGuardrailsCheck(t *testing.T) {
	g := testGuardrails()

	tests := []struct {
		name        string
		current     Money
		proposed    Money
		budgetType  BudgetType
		wantErr     bool
		wantLimit   GuardrailLimit
		description string
	}{
		{
			name:        "aumento dentro del factor permitido",
			current:     MoneyFromPesos(1000),
			proposed:    MoneyFromPesos(2500),
			budgetType:  BudgetDaily,
			description: "2,5x está por debajo de 3x",
		},
		{
			name:        "aumento exactamente en el factor límite",
			current:     MoneyFromPesos(1000),
			proposed:    MoneyFromPesos(3000),
			budgetType:  BudgetDaily,
			description: "3x justo es aceptable",
		},
		{
			name:       "aumento que excede el factor",
			current:    MoneyFromPesos(1000),
			proposed:   MoneyFromPesos(3001),
			budgetType: BudgetDaily,
			wantErr:    true,
			wantLimit:  LimitIncreaseFactor,
		},
		{
			name:        "error de tipeo: se agrega un cero",
			current:     MoneyFromPesos(3000),
			proposed:    MoneyFromPesos(30000),
			budgetType:  BudgetDaily,
			wantErr:     true,
			wantLimit:   LimitIncreaseFactor,
			description: "10x dispara el factor antes que el techo",
		},
		{
			name:       "resultado que excede el techo diario",
			current:    MoneyFromPesos(20000),
			proposed:   MoneyFromPesos(30000),
			budgetType: BudgetDaily,
			wantErr:    true,
			wantLimit:  LimitDailyCeiling,
		},
		{
			name:        "las bajas siempre se permiten",
			current:     MoneyFromPesos(50000),
			proposed:    MoneyFromPesos(1000),
			budgetType:  BudgetDaily,
			description: "aunque el actual ya supere el techo",
		},
		{
			name:        "el techo diario no aplica a presupuesto total",
			current:     MoneyFromPesos(20000),
			proposed:    MoneyFromPesos(30000),
			budgetType:  BudgetLifetime,
			description: "un total de $30.000 no es un gasto diario",
		},
		{
			name:        "sin presupuesto previo sólo aplica el techo",
			current:     Money{},
			proposed:    MoneyFromPesos(5000),
			budgetType:  BudgetDaily,
			description: "no hay base para calcular el factor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.Check(tt.current, tt.proposed, tt.budgetType)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("error inesperado (%s): %v", tt.description, err)
				}
				return
			}

			var ge *GuardrailError
			if !errors.As(err, &ge) {
				t.Fatalf("error = %v, esperaba un *GuardrailError", err)
			}
			if ge.Limit != tt.wantLimit {
				t.Errorf("Limit = %v, esperaba %v", ge.Limit, tt.wantLimit)
			}
			// El error debe transportar las cifras para armar el mensaje real.
			if ge.Attempted.Cents != tt.proposed.Cents {
				t.Errorf("Attempted = %d, esperaba %d", ge.Attempted.Cents, tt.proposed.Cents)
			}
			if ge.Max.IsZero() {
				t.Error("Max debería traer el máximo admitido, vino en cero")
			}
		})
	}
}

func TestGuardrailErrorIsInvalidInput(t *testing.T) {
	g := testGuardrails()

	err := g.Check(MoneyFromPesos(1000), MoneyFromPesos(99000), BudgetDaily)
	if KindOf(err) != KindInvalidInput {
		t.Errorf("KindOf = %v, esperaba %v", KindOf(err), KindInvalidInput)
	}
}

func TestGuardrailErrorMaxForFactor(t *testing.T) {
	g := testGuardrails()

	err := g.Check(MoneyFromPesos(1000), MoneyFromPesos(5000), BudgetDaily)

	var ge *GuardrailError
	if !errors.As(err, &ge) {
		t.Fatalf("esperaba *GuardrailError, vino %v", err)
	}
	// Con factor 3 y actual $1.000, el máximo admitido de una vez es $3.000.
	if ge.Max.Pesos() != 3000 {
		t.Errorf("Max = %v, esperaba 3000", ge.Max.Pesos())
	}
}

func TestDefaultGuardrails(t *testing.T) {
	g := DefaultGuardrails()

	if g.MaxIncreaseFactor != 3 {
		t.Errorf("MaxIncreaseFactor = %v, esperaba 3", g.MaxIncreaseFactor)
	}
	if g.MaxDailyBudget.Pesos() != 25000 {
		t.Errorf("MaxDailyBudget = %v, esperaba 25000", g.MaxDailyBudget.Pesos())
	}
}
