package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Rutas de descubrimiento del authorization server. El spec de MCP obliga al
// IdP a publicar al menos una de las dos; probamos la de OAuth primero porque
// es la que el spec nombra primero, y caemos a la de OpenID Connect para no
// atarnos a un proveedor.
const (
	wellKnownOAuthAS = "/.well-known/oauth-authorization-server"
	wellKnownOIDC    = "/.well-known/openid-configuration"
)

// maxMetadataBytes acota la lectura del documento de metadata. Es JSON chico y
// conocido; sin tope, un IdP comprometido o un proxy hostil podrían hacernos
// leer un cuerpo interminable.
const maxMetadataBytes = 1 << 20 // 1 MiB

// asMetadata es el subconjunto del metadata del authorization server que nos
// importa. `issuer` no es decorativo: se compara contra el configurado.
type asMetadata struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// DiscoverJWKSURI averigua dónde publica sus claves públicas el authorization
// server, partiendo únicamente del issuer.
//
// Derivarlo en vez de configurarlo aparte es deliberado: si el JWKS fuera una
// variable de entorno independiente, una configuración incoherente —issuer de
// un IdP, claves de otro— dejaría al server validando firmas contra un tercero
// sin que nada lo delate. Con una sola entrada de confianza eso no puede pasar.
func DiscoverJWKSURI(ctx context.Context, client *http.Client, issuer string) (string, error) {
	candidates, err := metadataURLs(issuer)
	if err != nil {
		return "", err
	}
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	for _, u := range candidates {
		meta, err := fetchMetadata(ctx, client, u)
		if err != nil {
			lastErr = err
			continue
		}

		// RFC 8414 §3.3: el issuer del documento debe coincidir exactamente
		// con el que usamos para pedirlo.
		if meta.Issuer != strings.TrimSuffix(issuer, "/") {
			return "", fmt.Errorf("oauth: el issuer del metadata (%q) no coincide con el configurado (%q)",
				meta.Issuer, issuer)
		}
		if meta.JWKSURI == "" {
			return "", fmt.Errorf("oauth: el metadata de %s no declara jwks_uri", u)
		}
		return meta.JWKSURI, nil
	}

	return "", fmt.Errorf("oauth: no se pudo descubrir el authorization server en %q: %w", issuer, lastErr)
}

// fetchMetadata trae y decodifica un documento de metadata.
func fetchMetadata(ctx context.Context, client *http.Client, u string) (asMetadata, error) {
	var meta asMetadata

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return meta, fmt.Errorf("oauth: armando el request a %s: %w", u, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return meta, fmt.Errorf("oauth: consultando %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return meta, fmt.Errorf("oauth: %s respondió %d", u, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if err != nil {
		return meta, fmt.Errorf("oauth: leyendo %s: %w", u, err)
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return meta, fmt.Errorf("oauth: %s no devolvió JSON válido: %w", u, err)
	}
	return meta, nil
}

// metadataURLs construye las URLs de descubrimiento para un issuer, en orden de
// preferencia.
//
// La forma no es una concatenación: RFC 8414 §3.1 manda insertar el segmento
// well-known entre el host y el path del issuer. Para un issuer con path,
// pegar el sufijo al final apuntaría a una URL que no existe.
func metadataURLs(issuer string) ([]string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(issuer), "/")
	if trimmed == "" {
		return nil, fmt.Errorf("oauth: el issuer está vacío")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("oauth: el issuer %q no es una URL válida: %w", issuer, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("oauth: el issuer debe usar https, recibido %q", issuer)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("oauth: el issuer %q no tiene host", issuer)
	}

	base := u.Scheme + "://" + u.Host
	path := strings.TrimSuffix(u.Path, "/")

	return []string{
		base + wellKnownOAuthAS + path,
		base + wellKnownOIDC + path,
	}, nil
}
