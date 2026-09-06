package oauth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// minJWKSRefresh es el piso del intervalo de refresco de las claves. El IdP
// sugiere una frecuencia por cabeceras de caché; este piso evita que una
// respuesta con un TTL absurdamente corto nos convierta en carga sostenida
// sobre WorkOS.
const minJWKSRefresh = 15 * time.Minute

// initialFetchTimeout acota la primera carga del JWKS. Sin él, `Register`
// espera indefinidamente a que el IdP esté listo y un WorkOS caído dejaría el
// arranque colgado para siempre en vez de fallar con un error legible.
const initialFetchTimeout = 10 * time.Second

// CachedKeySet mantiene en memoria las claves públicas del authorization
// server, refrescándolas en segundo plano.
//
// Existe porque validar un token no puede costar un viaje al IdP: si cada
// request de la clienta dependiera de que WorkOS responda, su latencia y su
// disponibilidad pasarían a ser las nuestras. El caché también absorbe la
// rotación de claves del IdP sin que haya que redeployar.
type CachedKeySet struct {
	cache *jwk.Cache
	uri   string
}

// NewCachedKeySet registra el JWKS y hace la primera carga de forma sincrónica.
//
// Que la primera carga sea bloqueante es intencional: si el IdP no responde al
// arrancar, preferimos no levantar antes que levantar un server incapaz de
// validar un solo token (Constitución: fallar explícito, nunca degradado).
//
// El ctx gobierna las goroutines de refresco: cancelarlo las detiene, así que
// debe vivir tanto como el servidor.
func NewCachedKeySet(ctx context.Context, jwksURI string, client *http.Client) (*CachedKeySet, error) {
	if jwksURI == "" {
		return nil, fmt.Errorf("oauth: falta la URI del JWKS")
	}
	if client == nil {
		client = http.DefaultClient
	}

	// El ctx de NewCache gobierna las goroutines de refresco y debe vivir tanto
	// como el servidor. El de Register sólo cubre la carga inicial, y por eso
	// lleva su propio plazo.
	cache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		return nil, fmt.Errorf("oauth: creando el caché de JWKS: %w", err)
	}

	first, cancel := context.WithTimeout(ctx, initialFetchTimeout)
	defer cancel()

	if err := cache.Register(first, jwksURI,
		jwk.WithHTTPClient(client),
		jwk.WithMinInterval(minJWKSRefresh),
	); err != nil {
		return nil, fmt.Errorf("oauth: no se pudo cargar el JWKS desde %s: %w", jwksURI, err)
	}

	return &CachedKeySet{cache: cache, uri: jwksURI}, nil
}

// Set devuelve las claves vigentes, servidas desde memoria.
func (c *CachedKeySet) Set(ctx context.Context) (jwk.Set, error) {
	set, err := c.cache.Lookup(ctx, c.uri)
	if err != nil {
		return nil, fmt.Errorf("oauth: leyendo el JWKS cacheado: %w", err)
	}
	return set, nil
}
