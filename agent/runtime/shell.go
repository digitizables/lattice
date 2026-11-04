package runtime

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
)

const shellTimeout = 60 * time.Second

func commandLine(name string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	if name != "" {
		parts = append(parts, name)
	}
	parts = append(parts, args...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func runShell(ctx context.Context, kind, line string) (exitCode int32, stdout, stderr string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 1, "", "empty command"
	}

	ctx, cancel := context.WithTimeout(ctx, shellTimeout)
	defer cancel()

	path, argv, err := shellInvocation(kind, line)
	if err != nil {
		return 1, "", err.Error()
	}

	cmd := exec.CommandContext(ctx, path, argv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if runErr == nil {
		return 0, stdout, stderr
	}
	if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
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

func runTask(ctx context.Context, t *agentv1.Task) (exitCode int32, stdout, stderr string) {
	name := strings.ToLower(strings.TrimSpace(t.GetName()))
	args := t.GetArgs()

	switch name {
	case "whoami":
		return runBuiltinWhoami()
	case "echo":
		return 0, strings.Join(args, " "), ""
	case "hostname":
		return runBuiltinHostname()
	case "shell", "run":
		return runShell(ctx, "default", commandLine("", args))
	case "cmd":
		return runShell(ctx, "cmd", commandLine("", args))
	case "powershell", "ps", "pwsh":
		return runShell(ctx, "powershell", commandLine("", args))
	case "bash", "sh":
		return runShell(ctx, name, commandLine("", args))
	default:
		// Treat unknown verbs as a native shell command line (dir, ls, Get-ChildItem, …).
		return runShell(ctx, "default", commandLine(t.GetName(), args))
	}
}
