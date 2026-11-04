//go:build unix

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

// colorPrelude defines ls/grep helpers so --color=auto runs under our PTY
// (GNU ls disables color unless --color is passed; interactive aliases are absent here).
const colorPrelude = `ls(){ command ls --color=auto "$@"; }; ` +
	`grep(){ command grep --color=auto "$@"; }; ` +
	`egrep(){ command grep -E --color=auto "$@"; }; ` +
	`fgrep(){ command grep -F --color=auto "$@"; }; `

func shellInvocation(kind, line string) (path string, argv []string, err error) {
	script := colorPrelude + line
	switch kind {
	case "default", "shell", "run", "sh":
		path = unixShellPath()
		return path, []string{"-c", script}, nil
	case "bash":
		path, err = exec.LookPath("bash")
		if err != nil {
			return "", nil, fmt.Errorf("bash not found: %w", err)
		}
		return path, []string{"-c", script}, nil
	case "cmd", "powershell", "ps", "pwsh":
		return "", nil, fmt.Errorf("%s is a Windows shell; use shell/bash/sh on this OS", kind)
	default:
		return "", nil, fmt.Errorf("unknown shell kind %q", kind)
	}
}

func unixShellPath() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		if path, err := exec.LookPath(sh); err == nil {
			return path
		}
		if filepath.IsAbs(sh) {
			if _, err := os.Stat(sh); err == nil {
				return sh
			}
		}
	}
	for _, candidate := range []string{"/bin/bash", "/bin/sh", "/usr/bin/bash", "/usr/bin/sh"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if path, err := exec.LookPath("sh"); err == nil {
		return path
	}
	return "/bin/sh"
}

func runBuiltinWhoami() (int32, string, string) {
	u, err := user.Current()
	if err != nil {
		return 1, "", err.Error()
	}
	return 0, u.Username, ""
}
