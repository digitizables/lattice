package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/digitizables/lattice/agent/runtime"
	"github.com/digitizables/lattice/agent/transport"
)

func main() {
	server := flag.String("server", "https://127.0.0.1:8443", "agent channel base URL")
	sleep := flag.Duration("sleep", 5*time.Second, "time between check-ins")
	insecure := flag.Bool("insecure", true, "skip TLS verify (needed for lab self-signed certs)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := &runtime.Agent{
		Sleep: *sleep,
		Transport: transport.NewHTTP(*server, transport.Options{
			InsecureSkipVerify: *insecure,
		}),
		Log: log,
	}
	if err := a.Run(ctx); err != nil && err != context.Canceled {
		log.Error("agent stopped", "err", err)
		os.Exit(1)
	}
}
