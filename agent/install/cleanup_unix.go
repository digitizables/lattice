//go:build unix

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func scheduleDelete(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	if _, err := os.Stat(path); err == nil {
		cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("sleep 1; rm -f %q", path))
		return cmd.Start()
	}
	return nil
}
