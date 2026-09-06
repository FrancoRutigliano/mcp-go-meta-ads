package mcp

import (
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

// MetadataConfig describe el documento Protected Resource Metadata (RFC 9728)
// que este server publica.
//
// Es la pieza que convierte un 401 en algo accionable: el challenge apunta acá,
// y acá el cliente aprende a qué authorization server ir a pedir un token.
type MetadataConfig struct {
	// Path es la ruta donde se publica, derivada del resource identifier.
	Path string

	// Resource es la URI canónica de este endpoint MCP.
	Resource string

	// AuthorizationServers son los emisores que este server acepta.
	AuthorizationServers []string

	// ScopesSupported son los permisos que el cliente puede pedir.
	ScopesSupported []string

	// ResourceName es el nombre legible que ve la persona al autorizar.
	ResourceName string
}

// RouterConfig describe el árbol de rutas del servidor HTTP.
type RouterConfig struct {
	// Endpoint es la ruta del endpoint MCP (normalmente "/mcp").
	Endpoint string

	// MCPHandler es el transporte MCP propiamente dicho.
	MCPHandler http.Handler

	// Auth define cómo se protege el endpoint.
	Auth AuthConfig

	// Metadata publica el documento de descubrimiento. Nil lo omite: sin
	// OAuth configurado, anunciar un authorization server que no existe
	// mandaría al cliente a un callejón sin salida.
	Metadata *MetadataConfig
}

// NewRouter compone el endpoint MCP, el documento de descubrimiento y el
// middleware de autenticación en un único handler.
//
// Vive acá y no en main() para que el armado —que es donde se decide qué queda
// protegido y qué queda público— sea verificable con tests.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(cfg.Endpoint, cfg.MCPHandler)

	if cfg.Metadata != nil {
		mux.Handle(cfg.Metadata.Path, server.NewProtectedResourceMetadataHandler(
			server.ProtectedResourceMetadataConfig{
				Resource:               cfg.Metadata.Resource,
				AuthorizationServers:   cfg.Metadata.AuthorizationServers,
				ScopesSupported:        cfg.Metadata.ScopesSupported,
				BearerMethodsSupported: []string{"header"},
				ResourceName:           cfg.Metadata.ResourceName,
			}))
	}

	return NewAuthMiddleware(cfg.Auth)(mux)
}
