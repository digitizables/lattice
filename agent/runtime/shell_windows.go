//go:build windows

package runtime

import (
	"fmt"
	"os/exec"
	"os/user"
)

func shellInvocation(kind, line string) (path string, argv []string, err error) {
	switch kind {
	case "default", "cmd":
		path, err = exec.LookPath("cmd.exe")
		if err != nil {
			return "", nil, fmt.Errorf("cmd.exe not found: %w", err)
		}
		return path, []string{"/D", "/C", line}, nil
	case "powershell", "ps", "pwsh":
		path, err = exec.LookPath("pwsh.exe")
		if err != nil {
			path, err = exec.LookPath("powershell.exe")
		}
		if err != nil {
			return "", nil, fmt.Errorf("PowerShell not found: %w", err)
		}
		return path, []string{"-NoProfile", "-NonInteractive", "-Command", line}, nil
	case "bash", "sh":
		return "", nil, fmt.Errorf("%s is not available as a Windows agent shell; use cmd or powershell", kind)
	default:
		return "", nil, fmt.Errorf("unknown shell kind %q", kind)
	}
}

func runBuiltinWhoami() (int32, string, string) {
	u, err := user.Current()
	if err != nil {
		return 1, "", err.Error()
	}
	return 0, u.Username, ""
}
