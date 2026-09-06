package mcp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mashats/meta-ads-manager/internal/adapters/oauth"
)

const (
	bearerPrefix = "Bearer "

	// wellKnownPrefix cubre los documentos de descubrimiento. Son públicos por
	// definición: el cliente los lee justo cuando todavía no tiene token, así
	// que protegerlos haría imposible arrancar el flujo de OAuth.
	wellKnownPrefix = "/.well-known/"
)

// TokenVerifier es lo único que el middleware necesita saber sobre OAuth.
// Mantenerlo como interfaz deja el transporte HTTP testeable sin criptografía
// ni red de por medio.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (oauth.Principal, error)
}

// AuthConfig describe cómo se protege el endpoint MCP.
//
// Las dos credenciales son independientes a propósito: durante la migración a
// OAuth conviven el bearer estático de siempre y los access tokens del IdP, y
// apagar el estático es sacar una variable de entorno. Si no hay ninguna de las
// dos, el endpoint queda abierto y el arranque lo anuncia.
type AuthConfig struct {
	// Verifier valida access tokens OAuth. Nil deshabilita OAuth.
	Verifier TokenVerifier

	// ResourceMetadataURL es la URL absoluta del documento RFC 9728 de este
	// server. Va en el WWW-Authenticate y es lo que le permite al cliente
	// descubrir el authorization server por su cuenta.
	ResourceMetadataURL string

	// RequiredScopes se anuncian en el challenge para que el cliente pida
	// exactamente los permisos que hacen falta, ni más ni menos.
	RequiredScopes []string

	// StaticToken es el bearer compartido heredado. Vacío lo deshabilita.
	StaticToken string
}

// NewAuthMiddleware construye el middleware de autenticación del endpoint MCP.
func NewAuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.enabled() || strings.HasPrefix(r.URL.Path, wellKnownPrefix) {
				next.ServeHTTP(w, r)
				return
			}

			presented := bearerToken(r.Header.Get("Authorization"))
			if presented == "" {
				cfg.challenge(w, http.StatusUnauthorized, "")
				return
			}

			if cfg.matchesStaticToken(presented) {
				next.ServeHTTP(w, r)
				return
			}

			if cfg.Verifier == nil {
				cfg.challenge(w, http.StatusUnauthorized, "")
				return
			}

			if _, err := cfg.Verifier.Verify(r.Context(), presented); err != nil {
				// Un token válido sin permisos suficientes es 403: con 401 el
				// cliente reintentaría autenticarse en loop sin llegar nunca a
				// pedir los scopes que le faltan.
				if errors.Is(err, oauth.ErrInsufficientScope) {
					cfg.challenge(w, http.StatusForbidden, "insufficient_scope")
					return
				}
				// Cualquier otro error —incluido uno inesperado— es rechazo.
				// Fallar cerrado: nunca interpretar una falla como permiso.
				cfg.challenge(w, http.StatusUnauthorized, "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// enabled indica si hay alguna credencial configurada.
func (cfg AuthConfig) enabled() bool {
	return cfg.Verifier != nil || cfg.StaticToken != ""
}

// matchesStaticToken compara contra el bearer heredado en tiempo constante,
// para no filtrar el secreto por diferencias de tiempo.
func (cfg AuthConfig) matchesStaticToken(presented string) bool {
	if cfg.StaticToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(cfg.StaticToken)) == 1
}

// challenge responde con el error y la cabecera WWW-Authenticate que el cliente
// necesita para saber qué hacer a continuación.
//
// Esta cabecera es el motivo por el que el conector de Claude fallaba: un 401
// pelado no le dice al cliente dónde está el metadata, así que no puede
// descubrir el authorization server y la conexión muere sin error accionable.
//
// El cuerpo es genérico a propósito: nunca repite el token presentado, porque
// termina en logs de proxies y de clientes.
func (cfg AuthConfig) challenge(w http.ResponseWriter, status int, errCode string) {
	var params []string
	if errCode != "" {
		params = append(params, fmt.Sprintf("error=%q", errCode))
	}
	if len(cfg.RequiredScopes) > 0 {
		params = append(params, fmt.Sprintf("scope=%q", strings.Join(cfg.RequiredScopes, " ")))
	}
	if cfg.ResourceMetadataURL != "" {
		params = append(params, fmt.Sprintf("resource_metadata=%q", cfg.ResourceMetadataURL))
	}

	challenge := "Bearer"
	if len(params) > 0 {
		challenge += " " + strings.Join(params, ", ")
	}

	w.Header().Set("WWW-Authenticate", challenge)
	http.Error(w, http.StatusText(status), status)
}

// bearerToken extrae la credencial de un header Authorization, o "" si el
// header falta, usa otro esquema o no trae valor.
func bearerToken(header string) string {
	if len(header) <= len(bearerPrefix) {
		return ""
	}
	if !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(header[len(bearerPrefix):])
}
