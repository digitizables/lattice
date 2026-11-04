package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds teamserver runtime settings.
type Config struct {
	DatabaseURL    string
	HTTPAddr       string
	AgentAddr      string
	ExtensionsDir  string
	CertDir        string
	UpdatesDir     string
	ModulesDir     string
	AgentCleartext bool
	// AllowLocalAgents permits implants on the teamserver host (default false).
	AllowLocalAgents bool
	Hostname         string
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cleartext, _ := strconv.ParseBool(envOr("LATTICE_AGENT_CLEARTEXT", "false"))
	allowLocal, _ := strconv.ParseBool(envOr("LATTICE_ALLOW_LOCAL_AGENTS", "false"))
	host, _ := os.Hostname()
	cfg := Config{
		DatabaseURL:      envOr("LATTICE_DATABASE_URL", "postgres://lattice:lattice@127.0.0.1:5432/lattice?sslmode=disable"),
		HTTPAddr:         envOr("LATTICE_HTTP_ADDR", ":8080"),
		AgentAddr:        envOr("LATTICE_AGENT_ADDR", ":8443"),
		ExtensionsDir:    envOr("LATTICE_EXTENSIONS_DIR", "extensions"),
		CertDir:          envOr("LATTICE_CERT_DIR", "data/certs"),
		UpdatesDir:       envOr("LATTICE_UPDATES_DIR", "data/updates"),
		ModulesDir:       envOr("LATTICE_MODULES_DIR", "data/modules"),
		AgentCleartext:   cleartext,
		AllowLocalAgents: allowLocal,
		Hostname:         host,
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
