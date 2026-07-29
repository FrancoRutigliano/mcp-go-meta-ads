package config

import (
	"errors"
	"strings"
	"testing"
)

// envMap returns a getenv-style function backed by a map, so tests never touch
// the real process environment.
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_FailsFastWhenTokenMissing(t *testing.T) {
	// Arrange
	getenv := envMap(map[string]string{
		"META_AD_ACCOUNT_ID": "act_331498724",
	})

	// Act
	_, err := Load(getenv)

	// Assert
	if !errors.Is(err, ErrMissingToken) {
		t.Fatalf("expected ErrMissingToken, got %v", err)
	}
}

func TestLoad_FailsFastWhenAccountMissing(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN": "secret-token",
	})

	_, err := Load(getenv)

	if !errors.Is(err, ErrMissingAccount) {
		t.Fatalf("expected ErrMissingAccount, got %v", err)
	}
}

func TestLoad_FailsWhenAccountHasNoActPrefix(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "331498724", // missing act_ prefix
	})

	_, err := Load(getenv)

	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount, got %v", err)
	}
}

func TestLoad_AppliesDefaults(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "act_331498724",
	})

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIVersion != defaultAPIVersion {
		t.Errorf("APIVersion default = %q, want %q", cfg.APIVersion, defaultAPIVersion)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port default = %q, want %q", cfg.Port, defaultPort)
	}
	if cfg.Endpoint != defaultEndpoint {
		t.Errorf("Endpoint default = %q, want %q", cfg.Endpoint, defaultEndpoint)
	}
	if cfg.AccountID != "act_331498724" {
		t.Errorf("AccountID = %q, want act_331498724", cfg.AccountID)
	}
}

func TestLoad_OverridesDefaults(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "act_1",
		"META_API_VERSION":   "v23.0",
		"PORT":               "9000",
	})

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIVersion != "v23.0" {
		t.Errorf("APIVersion = %q, want v23.0", cfg.APIVersion)
	}
	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want 9000", cfg.Port)
	}
}

func TestLoad_TrimsWhitespace(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "  secret-token  ",
		"META_AD_ACCOUNT_ID": "  act_1  ",
	})

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessToken != "secret-token" {
		t.Errorf("token not trimmed: %q", cfg.AccessToken)
	}
	if cfg.AccountID != "act_1" {
		t.Errorf("account not trimmed: %q", cfg.AccountID)
	}
}

func TestLoad_ThresholdsAndSufficiencyDefaults(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "act_1",
	})

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds.MinROAS != 2.0 || cfg.Thresholds.AvgTicket != 80000 {
		t.Errorf("threshold defaults inesperados: %+v", cfg.Thresholds)
	}
	if cfg.Sufficiency.MinPurchases != 1 || cfg.Sufficiency.MinDays != 7 {
		t.Errorf("sufficiency defaults inesperados: %+v", cfg.Sufficiency)
	}
}

func TestLoad_ThresholdOverrides(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "act_1",
		"ROAS_MIN":           "3",
		"AVG_TICKET":         "95000.5",
		"FREQUENCY_MAX":      "4",
		"MIN_DAYS":           "14",
	})

	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Thresholds.MinROAS != 3 {
		t.Errorf("MinROAS = %v, want 3", cfg.Thresholds.MinROAS)
	}
	if cfg.Thresholds.AvgTicket != 95000.5 {
		t.Errorf("AvgTicket = %v, want 95000.5", cfg.Thresholds.AvgTicket)
	}
	if cfg.Thresholds.MaxFrequency != 4 {
		t.Errorf("MaxFrequency = %v, want 4", cfg.Thresholds.MaxFrequency)
	}
	if cfg.Sufficiency.MinDays != 14 {
		t.Errorf("MinDays = %v, want 14", cfg.Sufficiency.MinDays)
	}
}

func TestLoad_InvalidNumericThresholdFailsFast(t *testing.T) {
	getenv := envMap(map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "act_1",
		"ROAS_MIN":           "no-es-un-numero",
	})

	_, err := Load(getenv)
	if err == nil {
		t.Fatal("expected error for invalid ROAS_MIN")
	}
}

// Constitución, Principio I: el token NUNCA debe aparecer en logs ni en
// representaciones de la config.
func TestConfig_StringRedactsToken(t *testing.T) {
	cfg := &Config{
		AccessToken: "super-secret-token-value",
		AccountID:   "act_331498724",
		APIVersion:  "v21.0",
		Port:        "8080",
	}

	out := cfg.String()

	if strings.Contains(out, "super-secret-token-value") {
		t.Fatalf("String() leaked the token: %q", out)
	}
	if !strings.Contains(out, "act_331498724") {
		t.Errorf("String() should still include non-secret fields: %q", out)
	}
}

// baseEnv devuelve el mínimo obligatorio para que Load no falle por credenciales.
func baseEnv(extra map[string]string) map[string]string {
	m := map[string]string{
		"META_TOKEN":         "secret-token",
		"META_AD_ACCOUNT_ID": "act_331498724",
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestLoad_GuardrailDefaults(t *testing.T) {
	cfg, err := Load(envMap(baseEnv(nil)))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if cfg.Guardrails.MaxIncreaseFactor != 3 {
		t.Errorf("MaxIncreaseFactor = %v, esperaba 3", cfg.Guardrails.MaxIncreaseFactor)
	}
	if cfg.Guardrails.MaxDailyBudget.Pesos() != 25000 {
		t.Errorf("MaxDailyBudget = %v, esperaba 25000", cfg.Guardrails.MaxDailyBudget.Pesos())
	}
}

func TestLoad_GuardrailOverrides(t *testing.T) {
	cfg, err := Load(envMap(baseEnv(map[string]string{
		"BUDGET_MAX_INCREASE_FACTOR": "2.5",
		"BUDGET_MAX_DAILY_ARS":       "40000",
	})))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if cfg.Guardrails.MaxIncreaseFactor != 2.5 {
		t.Errorf("MaxIncreaseFactor = %v, esperaba 2.5", cfg.Guardrails.MaxIncreaseFactor)
	}
	if cfg.Guardrails.MaxDailyBudget.Pesos() != 40000 {
		t.Errorf("MaxDailyBudget = %v, esperaba 40000", cfg.Guardrails.MaxDailyBudget.Pesos())
	}
}

func TestLoad_GuardrailInvalidValuesFailFast(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "factor no numérico", env: map[string]string{"BUDGET_MAX_INCREASE_FACTOR": "mucho"}},
		{name: "factor menor a 1", env: map[string]string{"BUDGET_MAX_INCREASE_FACTOR": "0.5"}},
		{name: "factor negativo", env: map[string]string{"BUDGET_MAX_INCREASE_FACTOR": "-3"}},
		{name: "techo no numérico", env: map[string]string{"BUDGET_MAX_DAILY_ARS": "mucha plata"}},
		{name: "techo cero", env: map[string]string{"BUDGET_MAX_DAILY_ARS": "0"}},
		{name: "techo negativo", env: map[string]string{"BUDGET_MAX_DAILY_ARS": "-1000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(envMap(baseEnv(tt.env)))
			if err == nil {
				t.Fatal("esperaba fallo de arranque explícito, no hubo error")
			}
			if !strings.Contains(err.Error(), "config:") {
				t.Errorf("el error debería identificar la config, vino: %v", err)
			}
		})
	}
}
