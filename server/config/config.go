package config

import (
	"fmt"
	"os"
)

// Config holds teamserver runtime settings.
type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	AgentAddr     string
	ExtensionsDir string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   envOr("LATTICE_DATABASE_URL", "postgres://lattice:lattice@127.0.0.1:5432/lattice?sslmode=disable"),
		HTTPAddr:      envOr("LATTICE_HTTP_ADDR", ":8080"),
		AgentAddr:     envOr("LATTICE_AGENT_ADDR", ":8443"),
		ExtensionsDir: envOr("LATTICE_EXTENSIONS_DIR", "extensions"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("LATTICE_DATABASE_URL is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
