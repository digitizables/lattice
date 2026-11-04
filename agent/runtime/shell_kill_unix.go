//go:build unix

package runtime

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func prepareShellCmd(cmd *exec.Cmd) {
	// Used for the non-PTY pipe fallback so cancel can kill children (ping, …).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killShellProcess(cmd *exec.Cmd, ptyFile *os.File) {
	if ptyFile != nil {
		_, _ = ptyFile.Write([]byte{0x03}) // Ctrl+C to the foreground process
	}
	if cmd.Process == nil {
		return
	}
	// pty.Start uses Setsid — pid is the session/process-group leader.
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGINT)
	time.Sleep(50 * time.Millisecond)
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
