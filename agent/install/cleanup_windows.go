//go:build windows

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func scheduleDelete(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// cmd delayed delete after this process exits
	script := fmt.Sprintf(`ping 127.0.0.1 -n 2 >nul & del /f /q "%s"`, path)
	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = os.Remove(path) // best-effort immediate delete if unlocked
	return nil
}
