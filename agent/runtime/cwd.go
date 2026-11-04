package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func defaultWorkingDir() string {
	if runtime.GOOS == "windows" {
		drive := os.Getenv("SystemDrive")
		if drive == "" {
			drive = "C:"
		}
		return drive + `\`
	}
	return "/"
}

func (a *Agent) ensureCwd() {
	if strings.TrimSpace(a.Cwd) == "" {
		a.Cwd = defaultWorkingDir()
	}
}

func (a *Agent) changeDirectory(target string) (string, error) {
	a.ensureCwd()
	target = strings.TrimSpace(target)
	if target == "" {
		return a.Cwd, nil
	}

	var next string
	if filepath.IsAbs(target) {
		next = filepath.Clean(target)
	} else {
		next = filepath.Clean(filepath.Join(a.Cwd, target))
	}

	info, err := os.Stat(next)
	if err != nil {
		return a.Cwd, fmt.Errorf("%s: %w", next, err)
	}
	if !info.IsDir() {
		return a.Cwd, fmt.Errorf("%s: not a directory", next)
	}
	a.Cwd = next
	return a.Cwd, nil
}
