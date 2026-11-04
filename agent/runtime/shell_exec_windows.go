//go:build windows

package runtime

import (
	"bytes"
	"os/exec"
)

// runShellProcess captures stdout/stderr on Windows. Color-aware tools that
// honor FORCE_COLOR / CLICOLOR_FORCE still emit ANSI for the UI to render.
func runShellProcess(cmd *exec.Cmd, stdout, stderr *bytes.Buffer) error {
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
