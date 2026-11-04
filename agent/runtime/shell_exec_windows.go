//go:build windows

package runtime

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"time"
)

// runShellProcess captures stdout/stderr on Windows. Color-aware tools that
// honor FORCE_COLOR / CLICOLOR_FORCE still emit ANSI for the UI to render.
func runShellProcess(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.Writer, onSnapshot func()) error {
	prepareShellCmd(cmd)
	cmd.Stdout = &snapWriter{w: stdout, snap: onSnapshot}
	cmd.Stderr = &snapWriter{w: stderr, snap: onSnapshot}

	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killShellProcess(cmd, nil)
		case <-done:
		}
	}()
	runErr := cmd.Wait()
	close(done)
	if onSnapshot != nil {
		onSnapshot()
	}
	return runErr
}

type snapWriter struct {
	w    io.Writer
	snap func()
	last time.Time
}

func (w *snapWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if w.snap != nil && (time.Since(w.last) >= 400*time.Millisecond || bytes.Contains(p, []byte{'\n'})) {
		w.snap()
		w.last = time.Now()
	}
	return n, err
}
