package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/digitizables/lattice/agent/identity"
	"github.com/digitizables/lattice/agent/install"
	"github.com/digitizables/lattice/agent/runtime"
	"github.com/digitizables/lattice/agent/transport"
	"github.com/kardianos/service"
)

func main() {
	server := flag.String("server", "https://127.0.0.1:8443", "agent channel base URL")
	sleep := flag.Duration("sleep", 5*time.Second, "time between check-ins")
	insecure := flag.Bool("insecure", true, "skip TLS verify (needed for lab self-signed certs)")
	idFile := flag.String("id-file", identity.DefaultIDFile(), "path used to persist the agent id across restarts")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	args := flag.Args()
	cmd := ""
	if len(args) > 0 {
		cmd = strings.ToLower(args[0])
	}

	opts := install.Options{
		Server:   *server,
		IDFile:   *idFile,
		Insecure: *insecure,
		Sleep:    sleep.String(),
	}

	switch cmd {
	case "install", "persist":
		method, err := install.Install(opts)
		if err != nil {
			log.Error("install failed", "err", err)
			fmt.Fprintln(os.Stderr, "hint: OS service needs elevation; logon-task and startup entry are user-level fallbacks")
			os.Exit(1)
		}
		fmt.Printf("installed persistence method %s (%q)\nbinary: %s\nid-file: %s\n",
			method, install.ServiceName(), install.BinaryPath(), *idFile)
		return
	case "uninstall":
		if err := install.UninstallService(); err != nil {
			log.Error("uninstall failed", "err", err)
			os.Exit(1)
		}
		fmt.Printf("removed persistence registration for %q (use cleanup to wipe files)\n", install.ServiceName())
		return
	case "cleanup":
		res, err := install.Cleanup()
		for _, step := range res.Steps {
			fmt.Println(step)
		}
		if err != nil {
			log.Error("cleanup finished with errors", "err", err)
			os.Exit(1)
		}
		fmt.Println("cleanup complete")
		os.Exit(0)
	}

	runAgent := func() {
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
			Log:         log,
			IDFile:      *idFile,
			ServerURL:   *server,
			InsecureTLS: *insecure,
		}
		if err := a.Run(ctx); err != nil && err != context.Canceled {
			log.Error("agent stopped", "err", err)
			os.Exit(1)
		}
	}

	prog := &install.Program{Run: runAgent}
	svc, err := install.NewService(opts, prog)
	if err != nil {
		log.Error("service", "err", err)
		os.Exit(1)
	}

	if !service.Interactive() {
		if err := svc.Run(); err != nil {
			log.Error("service run", "err", err)
			os.Exit(1)
		}
		return
	}

	runAgent()
}
