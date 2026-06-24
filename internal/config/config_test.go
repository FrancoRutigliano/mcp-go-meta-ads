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
