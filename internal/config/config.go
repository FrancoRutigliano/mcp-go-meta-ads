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
	"strconv"
	"strings"

	"github.com/mashats/meta-ads-manager/internal/domain"
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
	Thresholds  domain.Thresholds        // umbrales de negocio (Principio VIII)
	Sufficiency domain.SufficiencyPolicy // mínimos de muestra (Principio IX)
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

	thresholds, sufficiency, err := loadBusinessRules(getenv)
	if err != nil {
		return nil, err
	}

	return &Config{
		AccessToken: token,
		AccountID:   account,
		APIVersion:  apiVersion,
		Port:        port,
		Endpoint:    defaultEndpoint,
		Thresholds:  thresholds,
		Sufficiency: sufficiency,
	}, nil
}

// loadBusinessRules parte de los defaults y aplica overrides numéricos del
// entorno (Principios VIII y IX). Un valor presente pero inválido es un error
// de arranque explícito (fail-fast), no se ignora en silencio.
func loadBusinessRules(getenv func(string) string) (domain.Thresholds, domain.SufficiencyPolicy, error) {
	th := domain.DefaultThresholds()
	su := domain.DefaultSufficiency()

	var err error
	if th.MinROAS, err = floatEnv(getenv, "ROAS_MIN", th.MinROAS); err != nil {
		return th, su, err
	}
	if th.LinkCTRMin, err = floatEnv(getenv, "LINK_CTR_MIN", th.LinkCTRMin); err != nil {
		return th, su, err
	}
	if th.LinkCTRMax, err = floatEnv(getenv, "LINK_CTR_MAX", th.LinkCTRMax); err != nil {
		return th, su, err
	}
	if th.MaxFrequency, err = floatEnv(getenv, "FREQUENCY_MAX", th.MaxFrequency); err != nil {
		return th, su, err
	}
	if th.AvgTicket, err = floatEnv(getenv, "AVG_TICKET", th.AvgTicket); err != nil {
		return th, su, err
	}
	if su.MinPurchases, err = intEnv(getenv, "MIN_PURCHASES", su.MinPurchases); err != nil {
		return th, su, err
	}
	if su.MinImpressions, err = intEnv(getenv, "MIN_IMPRESSIONS", su.MinImpressions); err != nil {
		return th, su, err
	}
	if su.MinLinkClicks, err = intEnv(getenv, "MIN_LINK_CLICKS", su.MinLinkClicks); err != nil {
		return th, su, err
	}
	var minDays int64
	if minDays, err = intEnv(getenv, "MIN_DAYS", int64(su.MinDays)); err != nil {
		return th, su, err
	}
	su.MinDays = int(minDays)

	return th, su, nil
}

// floatEnv devuelve el valor del entorno como float64, o el default si está
// vacío. Si está presente pero no es numérico, devuelve error.
func floatEnv(getenv func(string) string, key string, def float64) (float64, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	v, parseErr := strconv.ParseFloat(raw, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("config: %s debe ser numérico (recibido: %q)", key, raw)
	}
	return v, nil
}

// intEnv devuelve el valor del entorno como int64, o el default si está vacío.
func intEnv(getenv func(string) string, key string, def int64) (int64, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	v, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("config: %s debe ser un entero (recibido: %q)", key, raw)
	}
	return v, nil
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
