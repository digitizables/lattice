//go:build unix

package runtime

import (
	"bytes"
	"io"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// runShellProcess runs cmd under a PTY so tools like ls emit colors and use a
// sensible column width. stdout and stderr are merged (normal for a TTY).
func runShellProcess(cmd *exec.Cmd, stdout, stderr *bytes.Buffer) error {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return runShellPipes(cmd, stdout, stderr)
	}

	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 100})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stdout, ptmx)
	}()

	runErr := cmd.Wait()
	_ = ptmx.Close()
	wg.Wait()
	return runErr
}

func runShellPipes(cmd *exec.Cmd, stdout, stderr *bytes.Buffer) error {
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
