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
	server := flag.String("server", "http://127.0.0.1:8443", "HTTP channel base URL")
	sleep := flag.Duration("sleep", 5*time.Second, "time between check-ins")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := &runtime.Agent{
		Sleep:     *sleep,
		Transport: transport.NewHTTP(*server),
		Log:       log,
	}
	if err := a.Run(ctx); err != nil && err != context.Canceled {
		log.Error("agent stopped", "err", err)
		os.Exit(1)
	}
}
