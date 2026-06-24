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
