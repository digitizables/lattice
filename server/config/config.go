package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds teamserver runtime settings.
type Config struct {
	DatabaseURL    string
	HTTPAddr       string
	AgentAddr      string
	TCPTLSAddr     string // optional second channel, e.g. :8444; empty disables
	ExtensionsDir  string
	CertDir        string
	UpdatesDir     string
	ModulesDir     string
	LootDir        string
	GeneratedDir   string
	SecretsDir     string
	AgentCleartext bool
	// AllowLocalAgents permits implants on the teamserver host (default false).
	AllowLocalAgents bool
	Hostname         string

	AgentToken string

	AuthDisabled      bool
	BootstrapUser     string
	BootstrapPassword string
	SessionTTLHours   int
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	cleartext, _ := strconv.ParseBool(envOr("LATTICE_AGENT_CLEARTEXT", "false"))
	allowLocal, _ := strconv.ParseBool(envOr("LATTICE_ALLOW_LOCAL_AGENTS", "false"))
	authDisabled, _ := strconv.ParseBool(envOr("LATTICE_AUTH_DISABLED", "false"))
	ttl, _ := strconv.Atoi(envOr("LATTICE_SESSION_TTL_HOURS", "12"))
	host, _ := os.Hostname()
	cfg := Config{
		DatabaseURL:       envOr("LATTICE_DATABASE_URL", "postgres://lattice:lattice@127.0.0.1:5432/lattice?sslmode=disable"),
		HTTPAddr:          envOr("LATTICE_HTTP_ADDR", ":8080"),
		AgentAddr:         envOr("LATTICE_AGENT_ADDR", ":8443"),
		TCPTLSAddr:        envOr("LATTICE_TCPTLS_ADDR", ":8444"),
		ExtensionsDir:     envOr("LATTICE_EXTENSIONS_DIR", "extensions"),
		CertDir:           envOr("LATTICE_CERT_DIR", "data/certs"),
		UpdatesDir:        envOr("LATTICE_UPDATES_DIR", "data/updates"),
		ModulesDir:        envOr("LATTICE_MODULES_DIR", "data/modules"),
		LootDir:           envOr("LATTICE_LOOT_DIR", "data/loot"),
		GeneratedDir:      envOr("LATTICE_GENERATED_DIR", "data/generated"),
		SecretsDir:        envOr("LATTICE_SECRETS_DIR", "data/secrets"),
		AgentCleartext:    cleartext,
		AllowLocalAgents:  allowLocal,
		Hostname:          host,
		AgentToken:        strings.TrimSpace(os.Getenv("LATTICE_AGENT_TOKEN")),
		AuthDisabled:      authDisabled,
		BootstrapUser:     envOr("LATTICE_BOOTSTRAP_OPERATOR", "admin"),
		BootstrapPassword: envOr("LATTICE_BOOTSTRAP_PASSWORD", "lattice-change-me"),
		SessionTTLHours:   ttl,
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
