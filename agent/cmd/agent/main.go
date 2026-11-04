package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/digitizables/lattice/agent/identity"
	"github.com/digitizables/lattice/agent/runtime"
	"github.com/digitizables/lattice/agent/transport"
)

func main() {
	server := flag.String("server", "https://127.0.0.1:8443", "agent channel base URL")
	sleep := flag.Duration("sleep", 5*time.Second, "time between check-ins")
	insecure := flag.Bool("insecure", true, "skip TLS verify (needed for lab self-signed certs)")
	idFile := flag.String("id-file", identity.DefaultIDFile(), "path used to persist the agent id across restarts")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	id, err := identity.Load(*idFile)
	if err != nil {
		log.Error("load agent id", "path", *idFile, "err", err)
		os.Exit(1)
	}
	if id != "" {
		log.Info("loaded agent id", "path", *idFile, "agent_id", id)
	} else {
		log.Info("no saved agent id; server will assign one", "path", *idFile)
	}

	a := &runtime.Agent{
		ID:    id,
		Sleep: *sleep,
		Transport: transport.NewHTTP(*server, transport.Options{
			InsecureSkipVerify: *insecure,
		}),
		Log:    log,
		IDFile: *idFile,
	}
	if err := a.Run(ctx); err != nil && err != context.Canceled {
		log.Error("agent stopped", "err", err)
		os.Exit(1)
	}
}
