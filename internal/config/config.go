// Package config carga y valida la configuración del servidor desde el entorno.
//
// Constitución, Principio I: el token de Meta sólo se obtiene de variables de
// entorno y nunca debe filtrarse en logs ni en representaciones de la config.
// Principio (fail-fast): si falta una credencial obligatoria, el arranque debe
// fallar de forma explícita, nunca arrancar en estado degradado.
package config

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultAPIVersion = "v21.0"
	defaultPort       = "8080"
	defaultEndpoint   = "/mcp"
)

// Errores de configuración. Son sentinelas para que el arranque (y los tests)
// puedan distinguir la causa con errors.Is.
var (
	ErrMissingToken   = errors.New("config: falta META_TOKEN")
	ErrMissingAccount = errors.New("config: falta META_AD_ACCOUNT_ID")
	ErrInvalidAccount = errors.New("config: META_AD_ACCOUNT_ID debe tener el prefijo 'act_'")
)

// Config contiene la configuración validada del servidor.
type Config struct {
	AccessToken string // secreto — no debe loguearse jamás
	AccountID   string
	APIVersion  string
	Port        string
	Endpoint    string
}

// Load lee la configuración usando la función getenv provista (normalmente
// os.Getenv), validando lo obligatorio y aplicando defaults. La inyección de
// getenv mantiene la función testeable sin tocar el entorno del proceso.
func Load(getenv func(string) string) (*Config, error) {
	token := strings.TrimSpace(getenv("META_TOKEN"))
	if token == "" {
		return nil, ErrMissingToken
	}

	account := strings.TrimSpace(getenv("META_AD_ACCOUNT_ID"))
	if account == "" {
		return nil, ErrMissingAccount
	}
	if !strings.HasPrefix(account, "act_") {
		return nil, fmt.Errorf("%w (recibido: %q)", ErrInvalidAccount, account)
	}

	apiVersion := strings.TrimSpace(getenv("META_API_VERSION"))
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}

	port := strings.TrimSpace(getenv("PORT"))
	if port == "" {
		port = defaultPort
	}

	return &Config{
		AccessToken: token,
		AccountID:   account,
		APIVersion:  apiVersion,
		Port:        port,
		Endpoint:    defaultEndpoint,
	}, nil
}

// String devuelve una representación segura de la config con el token redactado.
// Implementa fmt.Stringer para que cualquier log accidental nunca exponga el
// secreto (Constitución, Principio I).
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{AccountID:%s APIVersion:%s Port:%s Endpoint:%s AccessToken:%s}",
		c.AccountID, c.APIVersion, c.Port, c.Endpoint, redact(c.AccessToken),
	)
}

// redact enmascara un secreto dejando sólo una pista mínima para diagnóstico.
func redact(secret string) string {
	if secret == "" {
		return "<vacío>"
	}
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "…***"
}
