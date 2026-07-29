package domain

import (
	"errors"
	"fmt"
)

// Kind clasifica los errores del dominio por su semántica, de modo que las
// capas externas decidan cómo presentarlos (Constitución, Principios V y VII)
// sin acoplarse al detalle técnico de la causa.
type Kind string

const (
	KindUnknown      Kind = "unknown"
	KindUnauthorized Kind = "unauthorized" // credencial inválida o sin permisos
	KindRateLimited  Kind = "rate_limited" // la fuente pidió bajar el ritmo
	KindNotFound     Kind = "not_found"    // la entidad no existe
	KindInvalidInput Kind = "invalid_input"
	KindUpstream     Kind = "upstream" // fallo de la fuente de datos (Meta)
)

// Sentinelas del dominio para las condiciones de negocio que la capa de
// presentación necesita distinguir con errors.Is para dar un mensaje preciso
// (Constitución, Principios V y VII). Van envueltos en un *Error con su Kind,
// de modo que KindOf sigue funcionando.
var (
	// ErrNoBudgetChange: no se pidió ni monto absoluto ni ajuste relativo.
	ErrNoBudgetChange = errors.New("no se indicó ningún cambio de presupuesto")
	// ErrBothAmountAndPercent: se pidieron las dos formas a la vez.
	ErrBothAmountAndPercent = errors.New("se indicaron monto y porcentaje a la vez")
	// ErrNoBudgetToScale: se pidió un ajuste relativo sin presupuesto base.
	ErrNoBudgetToScale = errors.New("no hay presupuesto vigente sobre el cual calcular el ajuste")
	// ErrBudgetUnchanged: el monto pedido es igual al vigente.
	ErrBudgetUnchanged = errors.New("el presupuesto pedido es igual al actual")
	// ErrBudgetDrifted: el presupuesto cambió entre el propose y el confirm.
	ErrBudgetDrifted = errors.New("el presupuesto cambió desde que se armó la propuesta")
	// ErrBudgetLevelMismatch: se pidió escribir en el nivel equivocado.
	ErrBudgetLevelMismatch = errors.New("el presupuesto se administra en otro nivel")
	// ErrNoAdSets: la campaña no tiene conjuntos de anuncios.
	ErrNoAdSets = errors.New("la campaña no tiene conjuntos de anuncios")
	// ErrBudgetBelowMinimum: Meta rechazó el monto por ser menor a su mínimo.
	ErrBudgetBelowMinimum = errors.New("el presupuesto es menor al mínimo que permite Meta")
)

// Error es el error semántico del dominio. Conserva la causa técnica (para
// loguear del lado del servidor) separada de la clasificación que usa la capa
// de presentación para construir el mensaje en español.
type Error struct {
	Kind  Kind
	Op    string // operación de origen, sólo para logs (ej: "meta.ListCampaigns")
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Kind)
}

// Unwrap permite que errors.Is/As alcancen la causa subyacente.
func (e *Error) Unwrap() error { return e.Cause }

// NewError construye un error semántico del dominio.
func NewError(kind Kind, op string, cause error) *Error {
	return &Error{Kind: kind, Op: op, Cause: cause}
}

// KindOf extrae la clasificación semántica de un error, recorriendo la cadena
// de envoltura. Devuelve KindUnknown si no hay un *Error en la cadena.
func KindOf(err error) Kind {
	var de *Error
	if errors.As(err, &de) {
		return de.Kind
	}
	return KindUnknown
}
