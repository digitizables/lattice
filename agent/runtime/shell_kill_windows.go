//go:build windows

package runtime

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func prepareShellCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000200, // CREATE_NEW_PROCESS_GROUP
	}
}

func killShellProcess(cmd *exec.Cmd, _ *os.File) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	// /T kills the whole tree (cmd + ping, etc.).
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	hideWindow(kill)
	_ = kill.Run()
	_ = cmd.Process.Kill()
}
