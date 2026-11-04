package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupResult summarizes what was removed.
type CleanupResult struct {
	Steps []string
}

func (r *CleanupResult) add(format string, args ...any) {
	r.Steps = append(r.Steps, fmt.Sprintf(format, args...))
}

// Cleanup removes service registration, install files, config/id, and schedules
// self-deletion of the running binary when possible.
func Cleanup() (CleanupResult, error) {
	var res CleanupResult

	if err := UninstallService(); err != nil {
		res.add("service uninstall: %v", err)
	} else {
		res.add("service %q removed", ServiceName())
	}

	// Config / agent id
	cfg := ConfigDir()
	if err := os.RemoveAll(cfg); err != nil && !os.IsNotExist(err) {
		res.add("config dir %s: %v", cfg, err)
	} else {
		res.add("removed config dir %s", cfg)
	}

	// Install directory contents
	installDir := InstallDir()
	entries, err := os.ReadDir(installDir)
	if err == nil {
		for _, e := range entries {
			p := filepath.Join(installDir, e.Name())
			if err := os.RemoveAll(p); err != nil {
				res.add("remove %s: %v", p, err)
			} else {
				res.add("removed %s", p)
			}
		}
		if err := os.Remove(installDir); err != nil && !os.IsNotExist(err) {
			res.add("remove install dir %s: %v", installDir, err)
		} else {
			res.add("removed install dir %s", installDir)
		}
	} else if !os.IsNotExist(err) {
		res.add("install dir %s: %v", installDir, err)
	}

	exe, err := os.Executable()
	if err == nil {
		exe, _ = filepath.Abs(exe)
		if !strings.EqualFold(exe, BinaryPath()) {
			if err := scheduleDelete(exe); err != nil {
				res.add("schedule delete %s: %v", exe, err)
			} else {
				res.add("scheduled delete of running binary %s", exe)
			}
		} else {
			// Binary lived in install dir; already removed or locked — schedule delete.
			if err := scheduleDelete(exe); err != nil {
				res.add("schedule delete %s: %v", exe, err)
			} else {
				res.add("scheduled delete of service binary %s", exe)
			}
		}
	}

	// Give service manager a moment before the process exits.
	time.Sleep(300 * time.Millisecond)
	return res, nil
}
