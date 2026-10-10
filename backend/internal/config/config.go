package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL       string
	SessionSecret     string
	EncryptionKey     string
	AdminUsername     string
	AdminPasswordHash string
	Port              string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/crosscloudr?sslmode=disable"
	}

	required := map[string]string{
		"SESSION_SECRET": os.Getenv("SESSION_SECRET"),
		"ENCRYPTION_KEY": os.Getenv("ENCRYPTION_KEY"),
		"ADMIN_USERNAME": os.Getenv("ADMIN_USERNAME"),
	}

	for name, value := range required {
		if value == "" {
			return nil, fmt.Errorf("missing required environment variable: %s", name)
		}
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		return nil, fmt.Errorf("missing required environment variable: ADMIN_PASSWORD")
	}

	return &Config{
		DatabaseURL:       dbURL,
		SessionSecret:     required["SESSION_SECRET"],
		EncryptionKey:     required["ENCRYPTION_KEY"],
		AdminUsername:     required["ADMIN_USERNAME"],
		AdminPasswordHash: adminPassword,
		Port:              port,
	}, nil
}
