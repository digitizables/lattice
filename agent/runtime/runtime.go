package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/agent/transport"
)

// Agent is the reference implant loop.
type Agent struct {
	ID       string
	Sleep    time.Duration
	Transport *transport.HTTP
	Log      *slog.Logger
}

// Run check-ins until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if a.Log == nil {
		a.Log = slog.Default()
	}
	if a.Sleep <= 0 {
		a.Sleep = 5 * time.Second
	}

	for {
		if err := a.beat(ctx); err != nil {
			a.Log.Warn("check-in failed", "err", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(a.Sleep):
		}
	}
}

func (a *Agent) beat(ctx context.Context) error {
	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	resp, err := a.Transport.CheckIn(ctx, &agentv1.AgentCheckIn{
		AgentId:      a.ID,
		Hostname:     hostname,
		Username:     username,
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		SleepSeconds: uint64(a.Sleep / time.Second),
	})
	if err != nil {
		return err
	}
	a.ID = resp.GetAgentId()
	a.Log.Info("check-in ok", "agent_id", a.ID, "tasks", len(resp.GetTasks()))

	for _, t := range resp.GetTasks() {
		exitCode, stdout, stderr := runTask(t)
		_, err := a.Transport.PostResult(ctx, &agentv1.AgentTaskResult{
			AgentId:  a.ID,
			TaskId:   t.GetTaskId(),
			ExitCode: exitCode,
			Stdout:   stdout,
			Stderr:   stderr,
		})
		if err != nil {
			a.Log.Warn("post result failed", "task_id", t.GetTaskId(), "err", err)
		}
	}
	return nil
}

func runTask(t *agentv1.Task) (exitCode int32, stdout, stderr string) {
	switch strings.ToLower(t.GetName()) {
	case "whoami":
		u, err := user.Current()
		if err != nil {
			return 1, "", err.Error()
		}
		return 0, u.Username, ""
	case "echo":
		return 0, strings.Join(t.GetArgs(), " "), ""
	case "hostname":
		h, err := os.Hostname()
		if err != nil {
			return 1, "", err.Error()
		}
		return 0, h, ""
	default:
		return 1, "", fmt.Sprintf("unknown command %q", t.GetName())
	}
}
