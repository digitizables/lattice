//go:build unix

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func applyUpdate(data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
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
		if err2 := os.WriteFile(next, data, 0o755); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
		_ = os.Remove(tmp)
	}
	bak := exe + ".bak"
	_ = os.Remove(bak)
	script := fmt.Sprintf("sleep 1; mv -f %q %q; mv -f %q %q; exec %q", exe, bak, next, exe, exe)
	cmd := exec.Command("/bin/sh", "-c", script)
	return cmd.Start()
}
