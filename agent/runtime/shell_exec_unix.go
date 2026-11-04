//go:build unix

package runtime

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

// runShellProcess runs cmd under a PTY so tools like ls emit colors and use a
// sensible column width. stdout and stderr are merged (normal for a TTY).
// onSnapshot is called periodically with the captured output so far.
func runShellProcess(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.Writer, onSnapshot func()) error {
	// Let pty.Start set Setsid/Setctty; do not Setpgid first (conflicts on Linux).
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return runShellPipes(ctx, cmd, stdout, stderr, onSnapshot)
	}

	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 100})

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killShellProcess(cmd, ptmx)
		case <-done:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		lastSnap := time.Now()
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				_, _ = stdout.Write(buf[:n])
				if onSnapshot != nil && time.Since(lastSnap) >= 400*time.Millisecond {
					onSnapshot()
					lastSnap = time.Now()
				}
			}
			if readErr != nil {
				if onSnapshot != nil {
					onSnapshot()
				}
				return
			}
		}
	}()

	runErr := cmd.Wait()
	_ = ptmx.Close()
	close(done)
	wg.Wait()
	return runErr
}

func runShellPipes(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.Writer, onSnapshot func()) error {
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
