package config

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// wellKnownProtectedResource es el prefijo donde RFC 9728 manda publicar el
// documento de metadata del resource server.
const wellKnownProtectedResource = "/.well-known/oauth-protected-resource"

// ErrMissingPublicURL se devuelve cuando OAuth está habilitado pero no sabemos
// bajo qué URL nos ve el mundo.
var ErrMissingPublicURL = errors.New("config: falta PUBLIC_URL (obligatoria si se configura OAUTH_ISSUER)")

// OAuthConfig es la configuración del servidor en su rol de resource server.
//
// Sólo describe cómo validar tokens ajenos: quién los emite y para quién deben
// haber sido emitidos. La emisión es problema del IdP.
type OAuthConfig struct {
	// Issuer identifica al authorization server. Vacío deshabilita OAuth.
	Issuer string

	// Resource es la URI canónica de este endpoint MCP, y el valor que debe
	// aparecer en el claim `aud` de todo token que aceptemos.
	Resource string

	// RequiredScopes son los permisos exigidos. Vacío significa que alcanza con
	// estar autenticado.
	RequiredScopes []string
}

// Enabled indica si el servidor debe exigir access tokens OAuth.
func (c OAuthConfig) Enabled() bool { return c.Issuer != "" }

// MetadataPath es la ruta donde este server publica su Protected Resource
// Metadata.
//
// RFC 9728 §3.1: el path del resource va DESPUÉS del segmento well-known, no
// antes. Para un resource con path, concatenar al final apuntaría a una URL
// inexistente y el descubrimiento fallaría en silencio.
func (c OAuthConfig) MetadataPath() string {
	u, err := url.Parse(c.Resource)
	if err != nil || u.Host == "" {
		return wellKnownProtectedResource
	}

	p := strings.Trim(u.Path, "/")
	if p == "" {
		return wellKnownProtectedResource
	}
	return path.Join(wellKnownProtectedResource, p)
}

// MetadataURL es la URL absoluta de ese documento, tal como viaja en la
// cabecera WWW-Authenticate.
func (c OAuthConfig) MetadataURL() string {
	u, err := url.Parse(c.Resource)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + c.MetadataPath()
}

// loadOAuth arma la configuración de OAuth desde el entorno.
//
// Cuando OAUTH_ISSUER está presente, todo lo demás es obligatorio y se valida
// acá: una configuración de autenticación a medias es peor que ninguna, porque
// aparenta proteger algo que en realidad quedó abierto.
func loadOAuth(getenv func(string) string, endpoint string) (OAuthConfig, error) {
	issuer := strings.TrimSpace(getenv("OAUTH_ISSUER"))
	if issuer == "" {
		return OAuthConfig{}, nil
	}
	if err := requireHTTPS(issuer, "OAUTH_ISSUER"); err != nil {
		return OAuthConfig{}, err
	}

	publicURL := strings.TrimSpace(getenv("PUBLIC_URL"))
	if publicURL == "" {
		return OAuthConfig{}, ErrMissingPublicURL
	}
	if err := requireHTTPS(publicURL, "PUBLIC_URL"); err != nil {
		return OAuthConfig{}, err
	}

	return OAuthConfig{
		Issuer:         strings.TrimSuffix(issuer, "/"),
		Resource:       strings.TrimSuffix(publicURL, "/") + endpoint,
		RequiredScopes: parseScopes(getenv("OAUTH_REQUIRED_SCOPES")),
	}, nil
}

// requireHTTPS valida que un valor de configuración sea una URL absoluta con
// TLS. Sobre HTTP plano, cualquiera en el camino puede elegir contra qué claves
// validamos los tokens, y toda la cadena de confianza se cae.
func requireHTTPS(raw, name string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("config: %s no es una URL válida (%q): %w", name, raw, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("config: %s debe usar https (recibido %q)", name, raw)
	}
	if u.Host == "" {
		return fmt.Errorf("config: %s no tiene host (recibido %q)", name, raw)
	}
	return nil
}

// parseScopes acepta scopes separados por espacios (la forma de OAuth) o por
// comas (la forma en que suele escribirse una variable de entorno).
func parseScopes(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})

	var scopes []string
	for _, f := range fields {
		if s := strings.TrimSpace(f); s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}
