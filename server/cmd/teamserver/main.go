package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/digitizables/lattice/server/api"
	"github.com/digitizables/lattice/server/auth"
	"github.com/digitizables/lattice/server/config"
	"github.com/digitizables/lattice/server/core"
	latticecrypto "github.com/digitizables/lattice/server/crypto"
	"github.com/digitizables/lattice/server/extensions"
	"github.com/digitizables/lattice/server/httpx"
	"github.com/digitizables/lattice/server/listen"
	"github.com/digitizables/lattice/server/modules"
	"github.com/digitizables/lattice/server/secrets"
	"github.com/digitizables/lattice/server/store"
	"github.com/digitizables/lattice/server/updates"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	agentToken, generated, err := secrets.EnsureAgentToken(cfg.SecretsDir, cfg.AgentToken)
	if err != nil {
		log.Error("agent token", "err", err)
		os.Exit(1)
	}
	if generated {
		log.Warn("generated agent token", "path", filepath.Join(cfg.SecretsDir, "agent_token"), "hint", "set LATTICE_AGENT_TOKEN or bake agents with GenerateAgent")
	} else if cfg.AgentToken != "" {
		log.Info("agent token from environment")
	} else {
		log.Info("agent token loaded", "path", filepath.Join(cfg.SecretsDir, "agent_token"))
	}

	opAuth := auth.New(st, cfg.SessionTTLHours, cfg.AuthDisabled)
	if cfg.AuthDisabled {
		log.Warn("operator auth disabled", "env", "LATTICE_AUTH_DISABLED")
	} else {
		created, err := opAuth.BootstrapAdmin(ctx, cfg.BootstrapUser, cfg.BootstrapPassword)
		if err != nil {
			log.Error("bootstrap operator", "err", err)
			os.Exit(1)
		}
		if created {
			log.Warn("bootstrapped operator account",
				"username", cfg.BootstrapUser,
				"hint", "change LATTICE_BOOTSTRAP_PASSWORD; default is lattice-change-me")
		}
	}

	up, err := updates.New(cfg.UpdatesDir)
	if err != nil {
		log.Error("updates", "err", err)
		os.Exit(1)
	}
	log.Info("update catalog", "dir", cfg.UpdatesDir)

	mods, err := modules.New(cfg.ModulesDir)
	if err != nil {
		log.Error("modules", "err", err)
		os.Exit(1)
	}
	log.Info("module catalog", "dir", cfg.ModulesDir)

	lootStore, err := core.NewLootStore(cfg.LootDir, st)
	if err != nil {
		log.Error("loot", "err", err)
		os.Exit(1)
	}
	log.Info("loot store", "dir", cfg.LootDir)

	moduleRoot, _ := os.Getwd()
	c := core.New(st, up, mods, core.Options{
		AllowLocalAgents: cfg.AllowLocalAgents,
		ServerHostname:   cfg.Hostname,
		AgentToken:       agentToken,
		Loot:             lootStore,
	})
	if !cfg.AllowLocalAgents {
		log.Info("local agents denied", "hostname", cfg.Hostname, "hint", "set LATTICE_ALLOW_LOCAL_AGENTS=true to override")
		n, err := c.PurgeCommandCenterSessions(ctx)
		if err != nil {
			log.Error("purge command-center sessions", "err", err)
			os.Exit(1)
		}
		if n > 0 {
			log.Info("purged command-center host sessions", "count", n, "hostname", cfg.Hostname)
		}
	}
	if n, err := c.DeduplicateSessionsByHostname(ctx); err != nil {
		log.Error("dedupe sessions by hostname", "err", err)
		os.Exit(1)
	} else if n > 0 {
		log.Info("removed duplicate hostname sessions", "count", n)
	}
	mgr := listen.NewManager()

	deps := extensions.ChannelDeps{Core: c, Log: log, Cleartext: cfg.AgentCleartext}
	if !cfg.AgentCleartext {
		certFile, keyFile, err := latticecrypto.EnsureLabAgentCert(cfg.CertDir)
		if err != nil {
			log.Error("tls certs", "err", err)
			os.Exit(1)
		}
		deps.CertFile = certFile
		deps.KeyFile = keyFile
		log.Info("agent channel tls", "cert", certFile, "key", keyFile)
	} else {
		log.Warn("agent channel cleartext enabled", "env", "LATTICE_AGENT_CLEARTEXT")
	}

	loader := extensions.New(log)
	extensions.RegisterBuiltins(loader)
	extRuntime := extensions.NewRuntime(log, loader, mgr, deps, cfg.ExtensionsDir, cfg.AgentAddr)
	manifests, err := loader.Discover(cfg.ExtensionsDir)
	if err != nil {
		log.Error("discover extensions", "err", err)
		os.Exit(1)
	}
	if _, err := loader.StartChannels(ctx, mgr, deps, cfg.AgentAddr, manifests); err != nil {
		log.Error("start channels", "err", err)
		os.Exit(1)
	}

	connectServer := connect.NewServer()
	api.New(c, mgr, extRuntime, log, api.Options{
		Auth:         opAuth,
		AgentToken:   agentToken,
		GeneratedDir: cfg.GeneratedDir,
		ModuleRoot:   moduleRoot,
	}).Register(connectServer)

	mux := http.NewServeMux()
	connecthttp.Mount(mux, connectServer)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	handler := httpx.WithCORS(httpx.WithOperatorAuth(mux, opAuth))

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		Protocols:         protocols,
	}

	go func() {
		log.Info("operator API listening", "addr", cfg.HTTPAddr, "auth_disabled", cfg.AuthDisabled)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("operator API stopped", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	mgr.StopAll(shutdownCtx)
}
