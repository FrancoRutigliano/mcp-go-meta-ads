package domain

import "fmt"

// Defaults de los guardrails. El factor frena el error de tipeo puntual (agregar
// un cero); el techo diario frena el escalamiento acumulado por cambios sucesivos.
// El techo por defecto es coherente con el presupuesto declarado en la
// constitución (menor a USD 500/mes).
const (
	defaultMaxIncreaseFactor = 3.0
	defaultMaxDailyPesos     = 25000.0
)

// GuardrailLimit identifica cuál de los dos frenos se superó, para que el
// mensaje al usuario diga exactamente qué pasó.
type GuardrailLimit string

const (
	LimitIncreaseFactor GuardrailLimit = "factor_de_aumento"
	LimitDailyCeiling   GuardrailLimit = "techo_diario"
)

// GuardrailError es el error de límite de seguridad superado. Es un tipo y no un
// Kind nuevo porque el mensaje necesita transportar las cifras concretas: Kind
// clasifica semántica de transporte, no reglas de negocio con parámetros.
type GuardrailError struct {
	Limit     GuardrailLimit
	Current   Money // presupuesto vigente
	Attempted Money // monto que se intentó fijar
	Max       Money // máximo admitido para este caso
}

func (e *GuardrailError) Error() string {
	return fmt.Sprintf("límite de seguridad %s superado: intentó %d, máximo %d (unidades menores)",
		e.Limit, e.Attempted.Cents, e.Max.Cents)
}

// Guardrails acota cuánto puede crecer el gasto. Ambos valores son configurables
// por entorno sin tocar la lógica (FR-018).
type Guardrails struct {
	MaxIncreaseFactor float64 // múltiplo máximo del presupuesto vigente en una operación
	MaxDailyBudget    Money   // techo absoluto de gasto diario
}

// DefaultGuardrails devuelve los límites por defecto del negocio.
func DefaultGuardrails() Guardrails {
	return Guardrails{
		MaxIncreaseFactor: defaultMaxIncreaseFactor,
		MaxDailyBudget:    MoneyFromPesos(defaultMaxDailyPesos),
	}
}

// Check valida el monto propuesto contra ambos frenos.
//
// Las bajas siempre se permiten: bajar el gasto nunca es la operación riesgosa,
// incluso si el presupuesto vigente ya estuviera por encima del techo. El techo
// diario no aplica a presupuesto total, porque un total de $30.000 por toda la
// duración no es un gasto diario de $30.000.
func (g Guardrails) Check(current, proposed Money, t BudgetType) error {
	const op = "domain.Guardrails.Check"

	if proposed.Cents <= current.Cents {
		return nil
	}

	if !current.IsZero() && g.MaxIncreaseFactor > 0 {
		maxByFactor := Money{Cents: int64(float64(current.Cents) * g.MaxIncreaseFactor)}
		if proposed.Cents > maxByFactor.Cents {
			return NewError(KindInvalidInput, op, &GuardrailError{
				Limit:     LimitIncreaseFactor,
				Current:   current,
				Attempted: proposed,
				Max:       maxByFactor,
			})
		}
	}

	if t == BudgetDaily && !g.MaxDailyBudget.IsZero() && proposed.Cents > g.MaxDailyBudget.Cents {
		return NewError(KindInvalidInput, op, &GuardrailError{
			Limit:     LimitDailyCeiling,
			Current:   current,
			Attempted: proposed,
			Max:       g.MaxDailyBudget,
		})
	}

	return nil
}
