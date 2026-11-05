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

	"github.com/digitizables/lattice/agent/bake"
	"github.com/digitizables/lattice/agent/identity"
	"github.com/digitizables/lattice/agent/install"
	"github.com/digitizables/lattice/agent/runtime"
	"github.com/digitizables/lattice/agent/transport"
	"github.com/kardianos/service"
)

func main() {
	defaultServer := "https://127.0.0.1:8443"
	if bake.ServerURL != "" {
		defaultServer = bake.ServerURL
	}
	defaultSleep := 5 * time.Second
	if bake.SleepSeconds != "" {
		if sec, err := time.ParseDuration(bake.SleepSeconds + "s"); err == nil && sec > 0 {
			defaultSleep = sec
		}
	}
	defaultToken := strings.TrimSpace(os.Getenv("LATTICE_AGENT_TOKEN"))
	if bake.AgentToken != "" {
		defaultToken = bake.AgentToken
	}
	var defaultJitter uint
	if bake.JitterPercent != "" {
		fmt.Sscanf(bake.JitterPercent, "%d", &defaultJitter)
	}
	defaultInsecure := true
	if bake.InsecureTLS == "true" {
		defaultInsecure = true
	} else if bake.InsecureTLS == "false" {
		defaultInsecure = false
	}

	server := flag.String("server", defaultServer, "agent channel base URL")
	sleep := flag.Duration("sleep", defaultSleep, "time between check-ins")
	insecure := flag.Bool("insecure", defaultInsecure, "skip TLS verify (needed for lab self-signed certs)")
	idFile := flag.String("id-file", identity.DefaultIDFile(), "path used to persist the agent id across restarts")
	token := flag.String("token", defaultToken, "shared agent token (default env LATTICE_AGENT_TOKEN or bake)")
	jitter := flag.Uint("jitter", defaultJitter, "check-in sleep jitter percent 0-100")
	flag.Parse()

	jitterPct := *jitter
	if jitterPct > 100 {
		jitterPct = 100
	}

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
		fmt.Printf("removed persistence registration for %q (use cleanup to delete install files)\n", install.ServiceName())
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
			ID:            id,
			Sleep:         *sleep,
			Token:         strings.TrimSpace(*token),
			JitterPercent: uint32(jitterPct),
			Transport: transport.NewHTTP(*server, transport.Options{
				InsecureSkipVerify: *insecure,
				Token:              strings.TrimSpace(*token),
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
