package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverJWKSURI_ReturnsURIFromMetadata(t *testing.T) {
	// Arrange
	var gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"` + issuerOf(r) + `","jwks_uri":"` + issuerOf(r) + `/oauth2/jwks"}`))
	}))
	defer srv.Close()

	// Act
	uri, err := DiscoverJWKSURI(context.Background(), srv.Client(), srv.URL)

	// Assert
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if want := srv.URL + "/oauth2/jwks"; uri != want {
		t.Errorf("jwks_uri = %q, quiero %q", uri, want)
	}
	if want := "/.well-known/oauth-authorization-server"; gotPath != want {
		t.Errorf("consultó %q, quiero %q (RFC 8414)", gotPath, want)
	}
}

func TestDiscoverJWKSURI_FallsBackToOpenIDConfiguration(t *testing.T) {
	// Arrange: un IdP que sólo publica descubrimiento OIDC.
	var paths []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"` + issuerOf(r) + `","jwks_uri":"` + issuerOf(r) + `/jwks"}`))
	}))
	defer srv.Close()

	// Act
	uri, err := DiscoverJWKSURI(context.Background(), srv.Client(), srv.URL)

	// Assert
	if err != nil {
		t.Fatalf("debe caer al descubrimiento OIDC: %v", err)
	}
	if want := srv.URL + "/jwks"; uri != want {
		t.Errorf("jwks_uri = %q, quiero %q", uri, want)
	}
	if len(paths) != 2 || paths[0] != "/.well-known/oauth-authorization-server" {
		t.Errorf("orden de intentos = %v, quiero RFC 8414 primero", paths)
	}
}

func TestDiscoverJWKSURI_RejectsIssuerMismatch(t *testing.T) {
	// Arrange: el metadata dice ser otro emisor. Aceptarlo permitiría a un
	// host apuntar nuestras validaciones al JWKS de un tercero (mix-up).
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"https://otro-idp.example","jwks_uri":"https://otro-idp.example/jwks"}`))
	}))
	defer srv.Close()

	// Act
	_, err := DiscoverJWKSURI(context.Background(), srv.Client(), srv.URL)

	// Assert
	if err == nil {
		t.Fatal("un issuer que no coincide debe rechazarse")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("el error debe explicar el problema, got %q", err)
	}
}

func TestDiscoverJWKSURI_RejectsBadMetadata(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{name: "sin jwks_uri", body: `{"issuer":"%s"}`, code: http.StatusOK},
		{name: "json ilegible", body: `no soy json`, code: http.StatusOK},
		{name: "el IdP responde 500", body: `{}`, code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				w.Write([]byte(strings.ReplaceAll(tt.body, "%s", issuerOf(r))))
			}))
			defer srv.Close()

			if _, err := DiscoverJWKSURI(context.Background(), srv.Client(), srv.URL); err == nil {
				t.Error("quiero error, got nil")
			}
		})
	}
}

func TestMetadataURLs_FollowsRFC8414PathRule(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		want   string
	}{
		{
			name:   "issuer sin path",
			issuer: "https://tenant.authkit.app",
			want:   "https://tenant.authkit.app/.well-known/oauth-authorization-server",
		},
		{
			name:   "issuer con barra final",
			issuer: "https://tenant.authkit.app/",
			want:   "https://tenant.authkit.app/.well-known/oauth-authorization-server",
		},
		{
			// RFC 8414 §3.1: el path del issuer va DESPUÉS del well-known,
			// no antes. Concatenar ingenuamente daría una URL que no existe.
			name:   "issuer con path",
			issuer: "https://idp.example/tenant1",
			want:   "https://idp.example/.well-known/oauth-authorization-server/tenant1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := metadataURLs(tt.issuer)
			if err != nil {
				t.Fatalf("no esperaba error: %v", err)
			}
			if got[0] != tt.want {
				t.Errorf("URL RFC 8414 = %q, quiero %q", got[0], tt.want)
			}
		})
	}
}

func TestMetadataURLs_RejectsNonHTTPSIssuer(t *testing.T) {
	// El issuer viaja como base de confianza para traer las claves públicas.
	// Sobre HTTP plano, cualquiera en el camino elige con qué clave validamos.
	if _, err := metadataURLs("http://idp.example"); err == nil {
		t.Error("un issuer sin TLS debe rechazarse")
	}
}

// issuerOf reconstruye la URL base del servidor de prueba desde el request.
func issuerOf(r *http.Request) string {
	return "https://" + r.Host
}
