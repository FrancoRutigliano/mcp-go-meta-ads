package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// minorUnitsPerPeso es el factor de unidades menores de la moneda de la cuenta.
//
// La Meta Graph API expresa los presupuestos en unidades menores (centavos para
// ARS), así que $1.500,00 viaja como "150000". El factor es en realidad una
// propiedad de la moneda (currency_offset): hay monedas con factor 1. Acá se fija
// en 100 porque la constitución acota el alcance a una cuenta en ARS; si alguna
// vez se soporta otra moneda, hay que leer el offset de la cuenta en lugar de
// esta constante.
const minorUnitsPerPeso = 100

// maxPercentChange acota el ajuste relativo a un valor sensato. Un pedido por
// encima de esto es casi seguro un error de interpretación, y de todas formas
// los guardrails lo frenarían después.
const maxPercentChange = 1000

// Money es un monto expresado en unidades menores. Se usa int64 y no float64
// porque acá se decide gasto real e irreversible: un error de redondeo no es
// cosmético como en las métricas de lectura, es plata mal gastada.
type Money struct {
	Cents int64
}

// MoneyFromPesos construye un monto a partir de pesos.
func MoneyFromPesos(p float64) Money {
	return Money{Cents: int64(math.Round(p * minorUnitsPerPeso))}
}

// ParseMoneyMinorUnits interpreta el string en unidades menores que devuelve la
// Graph API. Un valor negativo o no numérico es un error, no un cero silencioso.
func ParseMoneyMinorUnits(raw string) (Money, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Money{}, fmt.Errorf("monto vacío")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("monto no numérico: %q", raw)
	}
	if v < 0 {
		return Money{}, fmt.Errorf("monto negativo: %q", raw)
	}
	return Money{Cents: v}, nil
}

// Pesos devuelve el monto en pesos, para presentación.
func (m Money) Pesos() float64 { return float64(m.Cents) / minorUnitsPerPeso }

// MinorUnits devuelve la representación que espera la Graph API.
func (m Money) MinorUnits() string { return strconv.FormatInt(m.Cents, 10) }

// IsZero indica que no hay monto (o que la entidad no tiene presupuesto).
func (m Money) IsZero() bool { return m.Cents == 0 }

// roundToPeso lleva el monto al peso entero más cercano, para que lo que el
// usuario aprueba sea exactamente lo que se aplica (SC-009).
func (m Money) roundToPeso() Money {
	return MoneyFromPesos(math.Round(m.Pesos()))
}

// BudgetType distingue el presupuesto diario del total por toda la duración.
type BudgetType string

const (
	BudgetDaily    BudgetType = "daily"
	BudgetLifetime BudgetType = "lifetime"
)

// Valid indica si el tipo está soportado.
func (t BudgetType) Valid() bool { return t == BudgetDaily || t == BudgetLifetime }

// Budget es el presupuesto de una entidad: un monto con su tipo.
type Budget struct {
	Type   BudgetType
	Amount Money
}

// BudgetLevel indica dónde se administra el presupuesto. Dentro de una misma
// campaña vive en un nivel o en el otro, nunca en los dos.
type BudgetLevel string

const (
	LevelCampaign BudgetLevel = "campaign"
	LevelAdSet    BudgetLevel = "adset"
)

// Valid indica si el nivel está soportado.
func (l BudgetLevel) Valid() bool { return l == LevelCampaign || l == LevelAdSet }

// BudgetChange es el cambio pedido por el usuario, expresado como monto absoluto
// o como ajuste relativo sobre el presupuesto vigente, nunca las dos formas a la
// vez (FR-011 a FR-015).
type BudgetChange struct {
	Amount  *Money   // monto absoluto pedido
	Percent *float64 // ajuste relativo en porcentaje (ej: 30 o -50)
}

// Validate verifica que el pedido tenga una única forma y sea sensato. No
// necesita conocer el presupuesto actual.
func (c BudgetChange) Validate() error {
	const op = "domain.BudgetChange.Validate"

	switch {
	case c.Amount != nil && c.Percent != nil:
		return NewError(KindInvalidInput, op, ErrBothAmountAndPercent)
	case c.Amount == nil && c.Percent == nil:
		return NewError(KindInvalidInput, op, ErrNoBudgetChange)
	}

	if c.Amount != nil {
		if c.Amount.Cents <= 0 {
			return NewError(KindInvalidInput, op,
				fmt.Errorf("el presupuesto debe ser mayor a cero"))
		}
		return nil
	}

	p := *c.Percent
	if p <= -100 {
		return NewError(KindInvalidInput, op,
			fmt.Errorf("un ajuste de %.0f%% dejaría el presupuesto en cero o menos", p))
	}
	if p > maxPercentChange {
		return NewError(KindInvalidInput, op,
			fmt.Errorf("un ajuste de %.0f%% es desmedido", p))
	}
	if p == 0 {
		return NewError(KindInvalidInput, op, ErrBudgetUnchanged)
	}
	return nil
}

// Apply calcula el monto resultante sobre el presupuesto vigente. El ajuste
// relativo se resuelve acá, en el servidor, contra el valor leído en el momento,
// y el resultado queda redondeado a peso entero.
func (c BudgetChange) Apply(current Money) (Money, error) {
	const op = "domain.BudgetChange.Apply"

	if err := c.Validate(); err != nil {
		return Money{}, err
	}

	var result Money
	if c.Amount != nil {
		result = c.Amount.roundToPeso()
	} else {
		if current.IsZero() {
			return Money{}, NewError(KindInvalidInput, op, ErrNoBudgetToScale)
		}
		factor := 1 + *c.Percent/100
		result = Money{Cents: int64(math.Round(float64(current.Cents) * factor))}.roundToPeso()
	}

	if result.Cents <= 0 {
		return Money{}, NewError(KindInvalidInput, op,
			fmt.Errorf("el presupuesto resultante debe ser mayor a cero"))
	}
	if result.Cents == current.Cents {
		return Money{}, NewError(KindInvalidInput, op, ErrBudgetUnchanged)
	}
	return result, nil
}
