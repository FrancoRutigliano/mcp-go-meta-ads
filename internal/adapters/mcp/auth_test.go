package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func doReq(h http.Handler, authHeader string) int {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestAuthMiddleware_OpenWhenNoToken(t *testing.T) {
	h := AuthMiddleware("", okHandler())
	if code := doReq(h, ""); code != http.StatusOK {
		t.Errorf("sin token configurado debe pasar, got %d", code)
	}
}

func TestAuthMiddleware_RejectsMissingHeader(t *testing.T) {
	h := AuthMiddleware("secreto", okHandler())
	if code := doReq(h, ""); code != http.StatusUnauthorized {
		t.Errorf("sin header debe dar 401, got %d", code)
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	h := AuthMiddleware("secreto", okHandler())
	if code := doReq(h, "Bearer otro"); code != http.StatusUnauthorized {
		t.Errorf("token incorrecto debe dar 401, got %d", code)
	}
}

func TestAuthMiddleware_RejectsMalformedHeader(t *testing.T) {
	h := AuthMiddleware("secreto", okHandler())
	if code := doReq(h, "secreto"); code != http.StatusUnauthorized {
		t.Errorf("header sin prefijo Bearer debe dar 401, got %d", code)
	}
}

func TestAuthMiddleware_AllowsCorrectToken(t *testing.T) {
	h := AuthMiddleware("secreto", okHandler())
	if code := doReq(h, "Bearer secreto"); code != http.StatusOK {
		t.Errorf("token correcto debe pasar, got %d", code)
	}
}
