package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Persistence method IDs returned by Install (see docs/operators/persistence.md).
const (
	MethodService   = "service"    // Windows Service / native OS service
	MethodLogonTask = "logon-task" // scheduled task at logon / systemd user unit
	MethodStartup   = "startup"    // HKCU Run / @reboot cron
)

// ServiceName is the OS service / task / Run value name (override with LATTICE_SERVICE_NAME).
func ServiceName() string {
	if v := strings.TrimSpace(os.Getenv("LATTICE_SERVICE_NAME")); v != "" {
		return v
	}
	return "LatticeAgent"
}

// ServiceDisplayName is shown in service managers.
func ServiceDisplayName() string {
	if v := strings.TrimSpace(os.Getenv("LATTICE_SERVICE_DISPLAY")); v != "" {
		return v
	}
	return "Lattice Agent"
}

// ServiceDescription describes the service.
func ServiceDescription() string {
	if v := strings.TrimSpace(os.Getenv("LATTICE_SERVICE_DESC")); v != "" {
		return v
	}
	return "Lattice reference agent (authorized lab / red-team use)"
}

// InstallDir is where the persistent agent binary lives.
func InstallDir() string {
	if v := strings.TrimSpace(os.Getenv("LATTICE_INSTALL_DIR")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "Lattice")
	}
	return "/usr/local/lib/lattice"
}

// BinaryName is the installed executable name.
func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "agent.exe"
	}
	return "agent"
}

// BinaryPath is InstallDir/BinaryName.
func BinaryPath() string {
	return filepath.Join(InstallDir(), BinaryName())
}

// ConfigDir holds agent.id and related state.
func ConfigDir() string {
	if v := strings.TrimSpace(os.Getenv("LATTICE_CONFIG_DIR")); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return filepath.Join(InstallDir(), "config")
	}
	return filepath.Join(dir, "lattice")
}
