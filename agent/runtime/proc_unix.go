//go:build unix

package runtime

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
