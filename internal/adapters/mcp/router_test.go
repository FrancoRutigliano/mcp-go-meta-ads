package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testEndpoint = "/mcp"
	testResource = "https://mcp.example.com/mcp"
	testIssuer   = "https://tenant.authkit.app"
	testMetaPath = "/.well-known/oauth-protected-resource/mcp"
)

// oauthRouter arma el router tal como lo compone el arranque real, con OAuth
// habilitado y un verificador de prueba.
func oauthRouter(t *testing.T) http.Handler {
	t.Helper()

	return NewRouter(RouterConfig{
		Endpoint:   testEndpoint,
		MCPHandler: okHandler(),
		Auth: AuthConfig{
			Verifier:            fakeVerifier{accept: testValidToken},
			ResourceMetadataURL: "https://mcp.example.com" + testMetaPath,
		},
		Metadata: &MetadataConfig{
			Path:                 testMetaPath,
			Resource:             testResource,
			AuthorizationServers: []string{testIssuer},
			ResourceName:         "meta-ads-manager",
		},
	})
}

func get(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// El recorrido completo que hace Claude al conectar un conector remoto: pega
// sin token, lee el challenge, busca el metadata que el challenge señala, y
// recién entonces vuelve con un access token.
func TestRouter_FullDiscoveryHandshake(t *testing.T) {
	h := oauthRouter(t)

	// 1. Request sin credenciales: 401 con la pista de dónde descubrir el IdP.
	rec := get(h, http.MethodPost, testEndpoint, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("primer request: code = %d, quiero %d", rec.Code, http.StatusUnauthorized)
	}

	challenge := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, testMetaPath) {
		t.Fatalf("el challenge %q debe apuntar al metadata", challenge)
	}

	// 2. El metadata que el challenge señala responde sin credenciales.
	rec = get(h, http.MethodGet, testMetaPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata: code = %d, quiero %d", rec.Code, http.StatusOK)
	}

	var meta struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("el metadata no es JSON válido: %v", err)
	}
	if meta.Resource != testResource {
		t.Errorf("resource = %q, quiero %q", meta.Resource, testResource)
	}
	if len(meta.AuthorizationServers) != 1 || meta.AuthorizationServers[0] != testIssuer {
		t.Errorf("authorization_servers = %v, quiero [%s]", meta.AuthorizationServers, testIssuer)
	}

	// 3. Con el token que ese IdP emitió, el endpoint responde.
	if rec := get(h, http.MethodPost, testEndpoint, testValidToken); rec.Code != http.StatusOK {
		t.Errorf("con token válido: code = %d, quiero %d", rec.Code, http.StatusOK)
	}
}

func TestRouter_WithoutOAuthServesNoMetadata(t *testing.T) {
	// Sin OAuth configurado no debe publicarse un metadata que apunte a un
	// authorization server inexistente: mandaría al cliente a un callejón.
	h := NewRouter(RouterConfig{
		Endpoint:   testEndpoint,
		MCPHandler: okHandler(),
		Auth:       AuthConfig{StaticToken: "secreto"},
	})

	if rec := get(h, http.MethodGet, testMetaPath, ""); rec.Code != http.StatusNotFound {
		t.Errorf("code = %d, quiero %d", rec.Code, http.StatusNotFound)
	}
}

func TestRouter_MCPEndpointStaysProtected(t *testing.T) {
	h := oauthRouter(t)

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{name: "sin token", token: "", want: http.StatusUnauthorized},
		{name: "token ajeno", token: "token-de-otro", want: http.StatusUnauthorized},
		{name: "token válido", token: testValidToken, want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := get(h, http.MethodPost, testEndpoint, tt.token).Code; got != tt.want {
				t.Errorf("code = %d, quiero %d", got, tt.want)
			}
		})
	}
}
