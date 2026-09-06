package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	testIssuer   = "https://test.authkit.app"
	testResource = "https://mcp.example.com/mcp"
)

// testKeys genera un par RSA y devuelve el set público (lo que publicaría el
// JWKS del IdP) junto con la clave privada usada para firmar en los tests.
func testKeys(t *testing.T) (jwk.Set, jwk.Key) {
	t.Helper()

	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generando clave RSA: %v", err)
	}

	priv, err := jwk.Import(raw)
	if err != nil {
		t.Fatalf("importando clave privada: %v", err)
	}
	if err := priv.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("seteando kid: %v", err)
	}
	if err := priv.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		t.Fatalf("seteando alg: %v", err)
	}

	pub, err := jwk.PublicKeyOf(priv)
	if err != nil {
		t.Fatalf("derivando clave pública: %v", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(pub); err != nil {
		t.Fatalf("armando el set: %v", err)
	}
	return set, priv
}

// tokenOpts describe un token a firmar. Los ceros significan "usar el valor
// válido por defecto", para que cada test sólo exprese lo que quiere romper.
type tokenOpts struct {
	issuer   string
	audience string
	subject  string
	scope    string
	expiry   time.Time
}

func signToken(t *testing.T, key jwk.Key, o tokenOpts) string {
	t.Helper()

	if o.issuer == "" {
		o.issuer = testIssuer
	}
	if o.audience == "" {
		o.audience = testResource
	}
	if o.subject == "" {
		o.subject = "user_123"
	}
	if o.expiry.IsZero() {
		o.expiry = time.Now().Add(time.Hour)
	}

	b := jwt.NewBuilder().
		Issuer(o.issuer).
		Audience([]string{o.audience}).
		Subject(o.subject).
		IssuedAt(time.Now()).
		Expiration(o.expiry)
	if o.scope != "" {
		b = b.Claim("scope", o.scope)
	}

	tok, err := b.Build()
	if err != nil {
		t.Fatalf("construyendo token: %v", err)
	}

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key))
	if err != nil {
		t.Fatalf("firmando token: %v", err)
	}
	return string(signed)
}

// staticKeys es un KeySet fijo, sin red, para los tests.
type staticKeys struct {
	set jwk.Set
	err error
}

func (s staticKeys) Set(context.Context) (jwk.Set, error) { return s.set, s.err }

func newTestVerifier(t *testing.T, set jwk.Set, requiredScopes []string) *Verifier {
	t.Helper()

	v, err := NewVerifier(Config{
		Issuer:         testIssuer,
		Resource:       testResource,
		RequiredScopes: requiredScopes,
	}, staticKeys{set: set})
	if err != nil {
		t.Fatalf("construyendo verifier: %v", err)
	}
	return v
}

func TestVerify_AcceptsValidToken(t *testing.T) {
	// Arrange
	set, priv := testKeys(t)
	v := newTestVerifier(t, set, nil)
	raw := signToken(t, priv, tokenOpts{subject: "user_abc"})

	// Act
	p, err := v.Verify(context.Background(), raw)

	// Assert
	if err != nil {
		t.Fatalf("un token válido debe aceptarse, got %v", err)
	}
	if p.Subject != "user_abc" {
		t.Errorf("Subject = %q, quiero %q", p.Subject, "user_abc")
	}
}

func TestVerify_RejectsInvalidTokens(t *testing.T) {
	set, priv := testKeys(t)

	// Una clave distinta a la publicada en el JWKS: firma válida, emisor equivocado.
	_, otraPriv := testKeys(t)

	tests := []struct {
		name  string
		token func() string
	}{
		{
			name:  "emisor distinto al configurado",
			token: func() string { return signToken(t, priv, tokenOpts{issuer: "https://malicioso.example"}) },
		},
		{
			name:  "audiencia de otro resource server",
			token: func() string { return signToken(t, priv, tokenOpts{audience: "https://otra-api.example"}) },
		},
		{
			name:  "token expirado",
			token: func() string { return signToken(t, priv, tokenOpts{expiry: time.Now().Add(-time.Minute)}) },
		},
		{
			name:  "firmado con una clave que no está en el JWKS",
			token: func() string { return signToken(t, otraPriv, tokenOpts{}) },
		},
		{
			name:  "cadena que no es un JWT",
			token: func() string { return "esto-no-es-un-token" },
		},
		{
			name:  "token vacío",
			token: func() string { return "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newTestVerifier(t, set, nil)

			_, err := v.Verify(context.Background(), tt.token())

			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("quiero ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestVerify_RejectsUnsignedToken(t *testing.T) {
	// Arrange: un JWT con alg=none es la trampa clásica; debe rechazarse igual
	// que cualquier firma inválida, nunca aceptarse por "no tener firma".
	set, _ := testKeys(t)
	v := newTestVerifier(t, set, nil)
	const algNone = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJpc3MiOiJodHRwczovL3Rlc3QuYXV0aGtpdC5hcHAiLCJzdWIiOiJhdGFjYW50ZSJ9."

	// Act
	_, err := v.Verify(context.Background(), algNone)

	// Assert
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("alg=none debe rechazarse, got %v", err)
	}
}

func TestVerify_ScopeEnforcement(t *testing.T) {
	set, priv := testKeys(t)

	tests := []struct {
		name       string
		required   []string
		tokenScope string
		wantErr    error
	}{
		{
			name:       "sin scopes requeridos alcanza con un token válido",
			required:   nil,
			tokenScope: "",
			wantErr:    nil,
		},
		{
			name:       "el scope requerido está presente",
			required:   []string{"ads:read"},
			tokenScope: "openid profile ads:read",
			wantErr:    nil,
		},
		{
			name:       "falta uno de los scopes requeridos",
			required:   []string{"ads:read", "ads:write"},
			tokenScope: "ads:read",
			wantErr:    ErrInsufficientScope,
		},
		{
			name:       "el token no trae ningún scope",
			required:   []string{"ads:read"},
			tokenScope: "",
			wantErr:    ErrInsufficientScope,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newTestVerifier(t, set, tt.required)
			raw := signToken(t, priv, tokenOpts{scope: tt.tokenScope})

			_, err := v.Verify(context.Background(), raw)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, quiero %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerify_ReportsScopesOfValidToken(t *testing.T) {
	// Arrange
	set, priv := testKeys(t)
	v := newTestVerifier(t, set, nil)
	raw := signToken(t, priv, tokenOpts{scope: "openid ads:read"})

	// Act
	p, err := v.Verify(context.Background(), raw)

	// Assert
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(p.Scopes) != 2 || p.Scopes[0] != "openid" || p.Scopes[1] != "ads:read" {
		t.Errorf("Scopes = %v, quiero [openid ads:read]", p.Scopes)
	}
}

func TestVerify_FailsClosedWhenJWKSUnavailable(t *testing.T) {
	// Arrange: si no podemos traer las claves del IdP no hay forma de validar
	// nada. Debe fallar cerrado, jamás dejar pasar el request.
	_, priv := testKeys(t)
	v, err := NewVerifier(Config{Issuer: testIssuer, Resource: testResource},
		staticKeys{err: errors.New("jwks caído")})
	if err != nil {
		t.Fatalf("construyendo verifier: %v", err)
	}
	raw := signToken(t, priv, tokenOpts{})

	// Act
	_, err = v.Verify(context.Background(), raw)

	// Assert
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("sin JWKS debe rechazar, got %v", err)
	}
}

func TestNewVerifier_RequiresIssuerAndResource(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "sin issuer", cfg: Config{Resource: testResource}},
		{name: "sin resource", cfg: Config{Issuer: testIssuer}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewVerifier(tt.cfg, staticKeys{set: jwk.NewSet()}); err == nil {
				t.Error("quiero error de configuración, got nil")
			}
		})
	}
}
