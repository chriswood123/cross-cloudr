package config

import (
	"os"
	"testing"
)

var requiredVars = []string{"SESSION_SECRET", "ENCRYPTION_KEY", "ADMIN_USERNAME", "ADMIN_PASSWORD"}

// unsetRequiredVars removes the required environment variables and registers
// a cleanup function to restore their original values.
func unsetRequiredVars(t *testing.T) {
	t.Helper()
	saved := make(map[string]string, len(requiredVars))
	for _, name := range requiredVars {
		saved[name] = os.Getenv(name)
		os.Unsetenv(name)
	}
	t.Cleanup(func() {
		for name, val := range saved {
			if val == "" {
				os.Unsetenv(name)
			} else {
				os.Setenv(name, val)
			}
		}
	})
}

func setRequiredVars(t *testing.T) {
	t.Helper()
	t.Setenv("SESSION_SECRET", "session-secret")
	t.Setenv("ENCRYPTION_KEY", "encryption-key")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
}

func TestLoad_MissingRequiredVars(t *testing.T) {
	unsetRequiredVars(t)

	if _, err := Load(); err == nil {
		t.Fatal("expected error when required env vars are missing")
	}
}

func TestLoad_MissingSingleVar(t *testing.T) {
	for _, missing := range requiredVars {
		t.Run(missing, func(t *testing.T) {
			setRequiredVars(t)
			os.Unsetenv(missing)

			if _, err := Load(); err == nil {
				t.Errorf("expected error when %s is missing", missing)
			}
		})
	}
}

func TestLoad_Success(t *testing.T) {
	setRequiredVars(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SessionSecret != "session-secret" {
		t.Errorf("expected session-secret, got %s", cfg.SessionSecret)
	}
	if cfg.EncryptionKey != "encryption-key" {
		t.Errorf("expected encryption-key, got %s", cfg.EncryptionKey)
	}
	if cfg.AdminUsername != "admin" {
		t.Errorf("expected admin, got %s", cfg.AdminUsername)
	}
	if cfg.AdminPassword != "password" {
		t.Errorf("expected password, got %s", cfg.AdminPassword)
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	setRequiredVars(t)
	os.Unsetenv("PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	setRequiredVars(t)
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
}

func TestLoad_CustomDatabaseURL(t *testing.T) {
	setRequiredVars(t)
	t.Setenv("DATABASE_URL", "postgres://custom:5432/db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://custom:5432/db" {
		t.Errorf("expected custom database URL, got %s", cfg.DatabaseURL)
	}
}
