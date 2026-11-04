package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/digitizables/lattice/server/api"
	"github.com/digitizables/lattice/server/config"
	"github.com/digitizables/lattice/server/core"
	latticecrypto "github.com/digitizables/lattice/server/crypto"
	"github.com/digitizables/lattice/server/extensions"
	"github.com/digitizables/lattice/server/listen"
	"github.com/digitizables/lattice/server/store"
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

	c := core.New(st)
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
	api.New(c, log).Register(connectServer)

	mux := http.NewServeMux()
	connecthttp.Mount(mux, connectServer)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		Protocols:         protocols,
	}

	go func() {
		log.Info("operator API listening", "addr", cfg.HTTPAddr)
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
