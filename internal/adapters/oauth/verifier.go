// Package oauth implementa el lado "resource server" de la autorización MCP.
//
// El spec de MCP (OAuth 2.1) reparte el trabajo en dos roles: el authorization
// server —que autentica a la persona y emite tokens— y el resource server —que
// sólo valida esos tokens—. Este paquete es únicamente lo segundo. El
// authorization server es un IdP externo (WorkOS AuthKit): no escribimos
// criptografía de emisión, ni pantallas de login, ni rotación de claves.
//
// La validación que hacemos acá no es una formalidad. El spec exige que el
// resource server compruebe que el token fue emitido *para él* (claim `aud`,
// RFC 8707): sin ese chequeo aceptaríamos cualquier token válido del IdP,
// incluido uno que un tercero obtuvo para otra aplicación y nos presenta acá
// para gastar el presupuesto de la cuenta de Meta (confused deputy).
package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// Errores sentinela: el middleware HTTP los traduce a códigos distintos
// (401 vs 403), así que la diferencia entre "no sos vos" y "sos vos pero no
// podés" tiene que sobrevivir el viaje hasta el handler.
var (
	// ErrInvalidToken cubre todo lo que impide confiar en el token: firma
	// inválida, emisor o audiencia equivocados, expirado, ilegible o ausente.
	ErrInvalidToken = errors.New("oauth: token inválido")

	// ErrInsufficientScope es un token legítimo al que le faltan permisos.
	ErrInsufficientScope = errors.New("oauth: scope insuficiente")
)

// KeySet es la fuente de claves públicas del authorization server (su JWKS).
// Es una interfaz para que el verificador no dependa de la red: en producción
// la implementa un caché con refresco automático, en los tests un set fijo.
type KeySet interface {
	Set(ctx context.Context) (jwk.Set, error)
}

// Config son los parámetros de validación. Issuer y Resource son obligatorios
// porque son justamente las dos preguntas que el token tiene que responder:
// quién lo emitió y para quién.
type Config struct {
	// Issuer es el identificador del authorization server, tal como aparece
	// en el claim `iss`. Ej: "https://tu-tenant.authkit.app".
	Issuer string

	// Resource es la URI canónica de este MCP server, que el IdP copia al
	// claim `aud` cuando el cliente la manda como parámetro `resource`.
	Resource string

	// RequiredScopes son los scopes que todo token debe traer. Vacío significa
	// que alcanza con estar autenticado, que es lo correcto mientras el server
	// atiende una sola cuenta de Meta.
	RequiredScopes []string
}

// Principal es la identidad detrás de un token ya validado.
type Principal struct {
	Subject string
	Scopes  []string
}

// Verifier valida access tokens JWT contra el JWKS del authorization server.
type Verifier struct {
	cfg  Config
	keys KeySet
}

// NewVerifier construye el verificador, fallando si falta algo sin lo cual la
// validación sería decorativa.
func NewVerifier(cfg Config, keys KeySet) (*Verifier, error) {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, errors.New("oauth: falta el issuer del authorization server")
	}
	if strings.TrimSpace(cfg.Resource) == "" {
		return nil, errors.New("oauth: falta el resource identifier de este MCP server")
	}
	if keys == nil {
		return nil, errors.New("oauth: falta la fuente de claves (JWKS)")
	}
	return &Verifier{cfg: cfg, keys: keys}, nil
}

// RequiredScopes devuelve una copia de los scopes exigidos, para que quien los
// publique en la cabecera WWW-Authenticate no pueda alterar la configuración.
func (v *Verifier) RequiredScopes() []string {
	return append([]string(nil), v.cfg.RequiredScopes...)
}

// Verify valida el token y devuelve la identidad que hay detrás.
//
// Los errores envuelven ErrInvalidToken o ErrInsufficientScope y nunca incluyen
// el token en sí: el mensaje termina en logs, y un access token en los logs es
// una credencial filtrada.
func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	if strings.TrimSpace(raw) == "" {
		return Principal{}, fmt.Errorf("%w: no se presentó ningún token", ErrInvalidToken)
	}

	// Sin claves no hay validación posible. Fallar cerrado: un IdP caído deja
	// el server inaccesible, nunca abierto.
	set, err := v.keys.Set(ctx)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: no se pudo obtener el JWKS del IdP: %w", ErrInvalidToken, err)
	}

	// jwt.Parse verifica la firma contra el set y valida iss, aud y exp. Al
	// exigir una clave del JWKS, un JWT con alg=none no tiene por dónde pasar.
	tok, err := jwt.Parse([]byte(raw),
		jwt.WithKeySet(set),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(v.cfg.Resource),
	)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	scopes := scopesOf(tok)
	if missing := missingScopes(v.cfg.RequiredScopes, scopes); len(missing) > 0 {
		return Principal{}, fmt.Errorf("%w: faltan %s", ErrInsufficientScope, strings.Join(missing, " "))
	}

	subject, _ := tok.Subject()
	return Principal{Subject: subject, Scopes: scopes}, nil
}

// scopesOf extrae los scopes del token. OAuth 2.1 los define como una cadena
// separada por espacios en el claim `scope`; algunos IdP usan `scp` como lista.
// Aceptamos ambos porque el costo es bajo y el modo de falla —creer que un
// token no tiene permisos que sí tiene— es confuso de diagnosticar.
func scopesOf(tok jwt.Token) []string {
	var raw string
	if err := tok.Get("scope", &raw); err == nil {
		return strings.Fields(raw)
	}

	var list []string
	if err := tok.Get("scp", &list); err == nil {
		return list
	}
	return nil
}

// missingScopes devuelve los scopes requeridos que el token no trae.
func missingScopes(required, granted []string) []string {
	if len(required) == 0 {
		return nil
	}

	have := make(map[string]struct{}, len(granted))
	for _, s := range granted {
		have[s] = struct{}{}
	}

	var missing []string
	for _, s := range required {
		if _, ok := have[s]; !ok {
			missing = append(missing, s)
		}
	}
	return missing
}
