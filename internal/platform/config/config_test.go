package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/webhooks?sslmode=disable")
	t.Setenv("HTTP_CLIENT_TIMEOUT_SECONDS", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q, want test", cfg.AppEnv)
	}
	if cfg.HTTPClientTimeout != 5*time.Second {
		t.Fatalf("HTTPClientTimeout = %s, want 5s", cfg.HTTPClientTimeout)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("PORT", "8080")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("HTTP_CLIENT_TIMEOUT_SECONDS", "5")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
