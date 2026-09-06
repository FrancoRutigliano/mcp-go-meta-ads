package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
)

// jwksServer publica un set de claves y cuenta cuántas veces se lo pidieron.
func jwksServer(t *testing.T, set jwk.Set, hits *atomic.Int64) *httptest.Server {
	t.Helper()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(set); err != nil {
			t.Errorf("serializando el JWKS: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCachedKeySet_ServesKeysFromTheIdP(t *testing.T) {
	// Arrange
	want, _ := testKeys(t)
	srv := jwksServer(t, want, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	keys, err := NewCachedKeySet(ctx, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("construyendo el caché: %v", err)
	}
	got, err := keys.Set(ctx)

	// Assert
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if got.Len() != want.Len() {
		t.Errorf("el set tiene %d claves, quiero %d", got.Len(), want.Len())
	}
	if _, ok := got.LookupKeyID("test-key-1"); !ok {
		t.Error("no encontré la clave publicada por el IdP")
	}
}

func TestCachedKeySet_DoesNotRefetchOnEveryCall(t *testing.T) {
	// Arrange: validar un token no puede costar un viaje al IdP, o cada
	// request del cliente arrastraría la latencia y la disponibilidad de WorkOS.
	set, _ := testKeys(t)
	var hits atomic.Int64
	srv := jwksServer(t, set, &hits)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys, err := NewCachedKeySet(ctx, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("construyendo el caché: %v", err)
	}

	// Act
	for range 20 {
		if _, err := keys.Set(ctx); err != nil {
			t.Fatalf("lookup falló: %v", err)
		}
	}

	// Assert
	if n := hits.Load(); n != 1 {
		t.Errorf("el IdP recibió %d pedidos, quiero 1 (el resto desde el caché)", n)
	}
}

func TestCachedKeySet_FailsFastWhenIdPUnreachable(t *testing.T) {
	// Arrange: si el JWKS no responde al arrancar, es preferible no levantar
	// antes que levantar sin poder validar un solo token.
	srv := jwksServer(t, jwk.NewSet(), nil)
	unreachable := srv.URL
	srv.Close()

	// httprc reintenta hasta que el contexto se agota. Un plazo corto acá deja
	// el test rápido sin cambiar la conducta que se está verificando.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Act
	_, err := NewCachedKeySet(ctx, unreachable, srv.Client())

	// Assert
	if err == nil {
		t.Error("un JWKS inalcanzable debe impedir el arranque")
	}
}

func TestCachedKeySet_VerifierAcceptsTokenEndToEnd(t *testing.T) {
	// Arrange: el camino completo tal como corre en producción — claves
	// traídas por HTTP desde el IdP y un token firmado con la privada.
	set, priv := testKeys(t)
	srv := jwksServer(t, set, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys, err := NewCachedKeySet(ctx, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("construyendo el caché: %v", err)
	}
	v, err := NewVerifier(Config{Issuer: testIssuer, Resource: testResource}, keys)
	if err != nil {
		t.Fatalf("construyendo el verifier: %v", err)
	}

	// Act
	p, err := v.Verify(ctx, signToken(t, priv, tokenOpts{subject: "user_e2e"}))

	// Assert
	if err != nil {
		t.Fatalf("el token debería validar: %v", err)
	}
	if p.Subject != "user_e2e" {
		t.Errorf("Subject = %q, quiero %q", p.Subject, "user_e2e")
	}
}
