package config

import (
	"errors"
	"testing"
)

// oauthEnv arma un entorno válido con las variables de OAuth encima del mínimo
// obligatorio, para que cada test sólo exprese lo que le interesa.
func oauthEnv(extra map[string]string) func(string) string {
	return envMap(baseEnv(extra))
}

func TestLoad_OAuthDisabledByDefault(t *testing.T) {
	// Sin OAUTH_ISSUER el servidor conserva su conducta previa, para que este
	// cambio no rompa un deploy en curso.
	cfg, err := Load(oauthEnv(nil))
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}

	if cfg.OAuth.Enabled() {
		t.Error("OAuth no debería estar habilitado sin configurar")
	}
}

func TestLoad_DerivesResourceFromPublicURL(t *testing.T) {
	// Arrange
	getenv := oauthEnv(map[string]string{
		"OAUTH_ISSUER": "https://tenant.authkit.app",
		"PUBLIC_URL":   "https://mcp.example.com",
	})

	// Act
	cfg, err := Load(getenv)

	// Assert
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if !cfg.OAuth.Enabled() {
		t.Fatal("OAuth debería estar habilitado")
	}
	// El resource identifier es la URI canónica del endpoint MCP: es el valor
	// que el cliente manda como `resource` y que el IdP copia al claim `aud`.
	if want := "https://mcp.example.com/mcp"; cfg.OAuth.Resource != want {
		t.Errorf("Resource = %q, quiero %q", cfg.OAuth.Resource, want)
	}
	if cfg.OAuth.Issuer != "https://tenant.authkit.app" {
		t.Errorf("Issuer = %q", cfg.OAuth.Issuer)
	}
}

func TestLoad_NormalizesPublicURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		want      string
	}{
		{name: "sin barra final", publicURL: "https://mcp.example.com", want: "https://mcp.example.com/mcp"},
		{name: "con barra final", publicURL: "https://mcp.example.com/", want: "https://mcp.example.com/mcp"},
		{name: "con espacios", publicURL: "  https://mcp.example.com  ", want: "https://mcp.example.com/mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(oauthEnv(map[string]string{
				"OAUTH_ISSUER": "https://tenant.authkit.app",
				"PUBLIC_URL":   tt.publicURL,
			}))
			if err != nil {
				t.Fatalf("no esperaba error: %v", err)
			}

			if cfg.OAuth.Resource != tt.want {
				t.Errorf("Resource = %q, quiero %q", cfg.OAuth.Resource, tt.want)
			}
		})
	}
}

func TestLoad_OAuthRequiresPublicURL(t *testing.T) {
	// Sin la URL pública no se puede construir el resource identifier, y sin él
	// no hay forma de validar que un token fue emitido para este server.
	_, err := Load(oauthEnv(map[string]string{
		"OAUTH_ISSUER": "https://tenant.authkit.app",
	}))

	if !errors.Is(err, ErrMissingPublicURL) {
		t.Fatalf("quiero ErrMissingPublicURL, got %v", err)
	}
}

func TestLoad_RejectsInsecureOAuthURLs(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "issuer sin TLS",
			env: map[string]string{
				"OAUTH_ISSUER": "http://tenant.authkit.app",
				"PUBLIC_URL":   "https://mcp.example.com",
			},
		},
		{
			name: "URL pública sin TLS",
			env: map[string]string{
				"OAUTH_ISSUER": "https://tenant.authkit.app",
				"PUBLIC_URL":   "http://mcp.example.com",
			},
		},
		{
			name: "issuer que no es una URL",
			env: map[string]string{
				"OAUTH_ISSUER": "tenant.authkit.app",
				"PUBLIC_URL":   "https://mcp.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Load(oauthEnv(tt.env)); err == nil {
				t.Error("quiero un error de configuración, got nil")
			}
		})
	}
}

func TestLoad_ParsesRequiredScopes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "vacío", raw: "", want: nil},
		{name: "uno solo", raw: "ads:read", want: []string{"ads:read"}},
		{name: "separados por espacio", raw: "ads:read ads:write", want: []string{"ads:read", "ads:write"}},
		{name: "separados por coma", raw: "ads:read,ads:write", want: []string{"ads:read", "ads:write"}},
		{name: "con espacios de más", raw: " ads:read ,  ads:write ", want: []string{"ads:read", "ads:write"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(oauthEnv(map[string]string{
				"OAUTH_ISSUER":          "https://tenant.authkit.app",
				"PUBLIC_URL":            "https://mcp.example.com",
				"OAUTH_REQUIRED_SCOPES": tt.raw,
			}))
			if err != nil {
				t.Fatalf("no esperaba error: %v", err)
			}

			got := cfg.OAuth.RequiredScopes
			if len(got) != len(tt.want) {
				t.Fatalf("RequiredScopes = %v, quiero %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("RequiredScopes = %v, quiero %v", got, tt.want)
				}
			}
		})
	}
}

func TestOAuthConfig_MetadataURL(t *testing.T) {
	// La URL que va en el WWW-Authenticate. RFC 9728 manda meter el path del
	// resource DESPUÉS del segmento well-known.
	cfg, err := Load(oauthEnv(map[string]string{
		"OAUTH_ISSUER": "https://tenant.authkit.app",
		"PUBLIC_URL":   "https://mcp.example.com",
	}))
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}

	want := "https://mcp.example.com/.well-known/oauth-protected-resource/mcp"
	if got := cfg.OAuth.MetadataURL(); got != want {
		t.Errorf("MetadataURL() = %q, quiero %q", got, want)
	}
}
