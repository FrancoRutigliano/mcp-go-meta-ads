package meta

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mashats/meta-ads-manager/internal/domain"
)

// graphError modela el objeto "error" de la Graph API.
type graphError struct {
	Message      string `json:"message"`
	Type         string `json:"type"`
	Code         int    `json:"code"`
	ErrorSubcode int    `json:"error_subcode"`
	FBTraceID    string `json:"fbtrace_id"`
}

type errorEnvelope struct {
	Error *graphError `json:"error"`
}

// translateError convierte una respuesta de error de Meta en un error semántico
// del dominio. La causa conserva el detalle técnico (código, tipo, fbtrace) para
// loguear del lado del servidor; NUNCA incluye el token (Constitución I y V).
func translateError(status int, ge *graphError, op string) error {
	kind := domain.KindUpstream
	switch {
	case status == http.StatusTooManyRequests:
		kind = domain.KindRateLimited
	case ge != nil:
		kind = kindFromCode(ge.Code, ge.ErrorSubcode)
	}

	// Las combinaciones de breakdown inválidas vienen como código 100 genérico;
	// las reconocemos por el mensaje y las tratamos como pedido inválido del
	// usuario, no como fallo del sistema (Principio V + brief feature 002).
	if kind == domain.KindUpstream && ge != nil && mentionsBreakdown(ge.Message) {
		kind = domain.KindInvalidInput
	}

	var cause error
	if ge != nil {
		cause = fmt.Errorf("graph error: http=%d code=%d subcode=%d type=%s fbtrace=%s msg=%q",
			status, ge.Code, ge.ErrorSubcode, ge.Type, ge.FBTraceID, ge.Message)
	} else {
		cause = fmt.Errorf("graph error: http=%d (sin cuerpo de error)", status)
	}
	return domain.NewError(kind, op, cause)
}

// mentionsBreakdown detecta errores de combinación de segmentación inválida.
func mentionsBreakdown(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "breakdown")
}

// kindFromCode mapea los códigos de error de Meta a la semántica del dominio.
// Referencia: Graph API error codes / rate limiting.
func kindFromCode(code, subcode int) domain.Kind {
	switch code {
	case 190, 102, 104, 2500: // token inválido / sesión / autenticación
		return domain.KindUnauthorized
	case 4, 17, 32, 613: // application/user/account rate limits
		return domain.KindRateLimited
	case 100:
		if subcode == 33 { // nodo inexistente o sin permiso de lectura
			return domain.KindNotFound
		}
		return domain.KindUpstream
	case 803: // alias/objeto inexistente
		return domain.KindNotFound
	}
	// Rango de "business use case" rate limiting.
	if code >= 80000 && code <= 80009 {
		return domain.KindRateLimited
	}
	return domain.KindUpstream
}
