//go:build windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func applyUpdate(data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	tmp, err := writeTempUpdate(data)
	if err != nil {
		return err
	}
	next := exe + ".new"
	_ = os.Remove(next)
	if err := os.Rename(tmp, next); err != nil {
		_ = os.Remove(tmp)
		// cross-device: copy
		if err2 := os.WriteFile(next, data, 0o755); err2 != nil {
			return err2
		}
		_ = os.Remove(tmp)
	}
	bak := exe + ".bak"
	_ = os.Remove(bak)

	// Replace after this process exits, then relaunch.
	script := fmt.Sprintf(
		`ping 127.0.0.1 -n 2 >nul & move /y "%s" "%s" >nul & move /y "%s" "%s" >nul & start "" "%s"`,
		exe, bak, next, exe, exe,
	)
	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
