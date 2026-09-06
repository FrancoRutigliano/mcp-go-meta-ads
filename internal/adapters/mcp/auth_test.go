package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mashats/meta-ads-manager/internal/adapters/oauth"
)

const (
	testResourceMetadataURL = "https://mcp.example.com/.well-known/oauth-protected-resource/mcp"
	testValidToken          = "token-valido"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

// fakeVerifier acepta un único token y devuelve el error configurado para
// cualquier otro. Mantiene al middleware bajo prueba sin criptografía ni red.
type fakeVerifier struct {
	accept string
	err    error
}

func (f fakeVerifier) Verify(_ context.Context, raw string) (oauth.Principal, error) {
	if raw == f.accept {
		return oauth.Principal{Subject: "user_1", Scopes: []string{"ads:read"}}, nil
	}
	if f.err != nil {
		return oauth.Principal{}, f.err
	}
	return oauth.Principal{}, oauth.ErrInvalidToken
}

func do(h http.Handler, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func oauthMiddleware(t *testing.T, cfg AuthConfig) http.Handler {
	t.Helper()

	if cfg.Verifier == nil {
		cfg.Verifier = fakeVerifier{accept: testValidToken}
	}
	if cfg.ResourceMetadataURL == "" {
		cfg.ResourceMetadataURL = testResourceMetadataURL
	}
	return NewAuthMiddleware(cfg)(okHandler())
}

func TestAuth_OpenWhenNothingConfigured(t *testing.T) {
	// Sin OAuth ni token estático el endpoint queda abierto. Es la conducta
	// histórica; el arranque la registra de forma explícita.
	h := NewAuthMiddleware(AuthConfig{})(okHandler())

	if got := do(h, "").Code; got != http.StatusOK {
		t.Errorf("code = %d, quiero %d", got, http.StatusOK)
	}
}

func TestAuth_AcceptsValidOAuthToken(t *testing.T) {
	h := oauthMiddleware(t, AuthConfig{})

	if got := do(h, "Bearer "+testValidToken).Code; got != http.StatusOK {
		t.Errorf("code = %d, quiero %d", got, http.StatusOK)
	}
}

func TestAuth_RejectsBadOAuthTokens(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "sin header", header: ""},
		{name: "token desconocido", header: "Bearer otro-token"},
		{name: "sin el prefijo Bearer", header: testValidToken},
		{name: "esquema equivocado", header: "Basic " + testValidToken},
		{name: "Bearer sin valor", header: "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := oauthMiddleware(t, AuthConfig{})

			if got := do(h, tt.header).Code; got != http.StatusUnauthorized {
				t.Errorf("code = %d, quiero %d", got, http.StatusUnauthorized)
			}
		})
	}
}

// Este es el test que corresponde al bug que rompía el conector: un 401 sin
// WWW-Authenticate no le dice a Claude dónde descubrir el flujo de OAuth, y la
// conexión falla en silencio y sin error accionable.
func TestAuth_ChallengePointsToResourceMetadata(t *testing.T) {
	// Arrange
	h := oauthMiddleware(t, AuthConfig{})

	// Act
	rec := do(h, "")

	// Assert
	challenge := rec.Header().Get("WWW-Authenticate")
	if challenge == "" {
		t.Fatal("un 401 sin WWW-Authenticate deja al cliente sin forma de descubrir el authorization server")
	}
	if !strings.HasPrefix(challenge, "Bearer ") {
		t.Errorf("el challenge debe usar el esquema Bearer, got %q", challenge)
	}
	want := `resource_metadata="` + testResourceMetadataURL + `"`
	if !strings.Contains(challenge, want) {
		t.Errorf("challenge = %q, debe contener %s", challenge, want)
	}
}

func TestAuth_ChallengeAdvertisesRequiredScopes(t *testing.T) {
	// Arrange
	h := oauthMiddleware(t, AuthConfig{RequiredScopes: []string{"ads:read", "ads:write"}})

	// Act
	challenge := do(h, "").Header().Get("WWW-Authenticate")

	// Assert
	if !strings.Contains(challenge, `scope="ads:read ads:write"`) {
		t.Errorf("challenge = %q, debe anunciar los scopes requeridos", challenge)
	}
}

func TestAuth_OmitsScopeWhenNoneRequired(t *testing.T) {
	// Un scope="" vacío es peor que ausente: le pide al cliente que solicite
	// la cadena vacía en vez de dejarlo elegir según el metadata.
	h := oauthMiddleware(t, AuthConfig{})

	if challenge := do(h, "").Header().Get("WWW-Authenticate"); strings.Contains(challenge, "scope=") {
		t.Errorf("challenge = %q, no debe traer scope si no hay ninguno requerido", challenge)
	}
}

func TestAuth_InsufficientScopeIsForbiddenNotUnauthorized(t *testing.T) {
	// Arrange: un token legítimo al que le faltan permisos no es un problema de
	// autenticación. Un 401 mandaría al cliente a reautenticarse en loop.
	h := oauthMiddleware(t, AuthConfig{
		Verifier:       fakeVerifier{accept: "otro", err: oauth.ErrInsufficientScope},
		RequiredScopes: []string{"ads:write"},
	})

	// Act
	rec := do(h, "Bearer token-sin-permisos")

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, quiero %d", rec.Code, http.StatusForbidden)
	}
	challenge := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, `error="insufficient_scope"`) {
		t.Errorf("challenge = %q, debe traer error=insufficient_scope", challenge)
	}
	if !strings.Contains(challenge, `scope="ads:write"`) {
		t.Errorf("challenge = %q, debe decir qué scope hace falta", challenge)
	}
}

func TestAuth_StaticTokenStillAcceptedDuringMigration(t *testing.T) {
	// El bearer estático sigue sirviendo mientras la clienta migra a OAuth.
	// El arranque avisa que está habilitado.
	h := oauthMiddleware(t, AuthConfig{StaticToken: "secreto-legacy"})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{name: "token estático", header: "Bearer secreto-legacy", want: http.StatusOK},
		{name: "token OAuth", header: "Bearer " + testValidToken, want: http.StatusOK},
		{name: "ninguno de los dos", header: "Bearer nada", want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(h, tt.header).Code; got != tt.want {
				t.Errorf("code = %d, quiero %d", got, tt.want)
			}
		})
	}
}

func TestAuth_LegacyStaticTokenOnly(t *testing.T) {
	// Sin OAuth configurado el middleware conserva la conducta previa.
	h := NewAuthMiddleware(AuthConfig{StaticToken: "secreto"})(okHandler())

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{name: "token correcto", header: "Bearer secreto", want: http.StatusOK},
		{name: "token incorrecto", header: "Bearer otro", want: http.StatusUnauthorized},
		{name: "sin header", header: "", want: http.StatusUnauthorized},
		{name: "sin prefijo Bearer", header: "secreto", want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(h, tt.header).Code; got != tt.want {
				t.Errorf("code = %d, quiero %d", got, tt.want)
			}
		})
	}
}

func TestAuth_NeverEchoesTheToken(t *testing.T) {
	// Un token rechazado no puede volver en la respuesta: el cuerpo termina en
	// logs de cliente y de proxies.
	const secreto = "token-que-no-debe-aparecer"
	h := oauthMiddleware(t, AuthConfig{})

	rec := do(h, "Bearer "+secreto)

	if strings.Contains(rec.Body.String(), secreto) {
		t.Error("la respuesta filtró el token presentado")
	}
	if strings.Contains(rec.Header().Get("WWW-Authenticate"), secreto) {
		t.Error("el challenge filtró el token presentado")
	}
}

func TestAuth_MetadataEndpointStaysPublic(t *testing.T) {
	// El documento de metadata es lo que el cliente lee JUSTO cuando todavía no
	// tiene token. Si lo protegemos, el descubrimiento no puede arrancar nunca.
	h := oauthMiddleware(t, AuthConfig{})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, quiero %d: el metadata debe ser público", rec.Code, http.StatusOK)
	}
}

func TestAuth_VerifierErrorsMapToStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "token inválido", err: oauth.ErrInvalidToken, want: http.StatusUnauthorized},
		{name: "scope insuficiente", err: oauth.ErrInsufficientScope, want: http.StatusForbidden},
		{
			// Un error inesperado no puede interpretarse como permiso.
			name: "error desconocido",
			err:  errors.New("algo raro pasó"),
			want: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := oauthMiddleware(t, AuthConfig{Verifier: fakeVerifier{accept: "nada", err: tt.err}})

			if got := do(h, "Bearer x").Code; got != tt.want {
				t.Errorf("code = %d, quiero %d", got, tt.want)
			}
		})
	}
}
