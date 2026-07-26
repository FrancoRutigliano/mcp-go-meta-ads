package mcp

import (
	"crypto/subtle"
	"net/http"
)

// AuthMiddleware protege el endpoint MCP con un bearer token compartido.
//
// Constitución (superficie mínima): el endpoint HTTP es público en Internet;
// sin auth cualquiera con la URL vería el gasto de la cuenta. Si token está
// vacío, el middleware deja pasar (modo abierto) — esa decisión se registra de
// forma explícita en el arranque, nunca en silencio.
func AuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !validBearer(r.Header.Get("Authorization"), token) {
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validBearer compara el header Authorization contra "Bearer <token>" en tiempo
// constante para no filtrar el secreto por diferencias de tiempo.
func validBearer(header, token string) bool {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	got := header[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
