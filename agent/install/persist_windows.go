//go:build windows

package install

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func taskName() string {
	return ServiceName()
}

// Install prefers an OS service; on access denial tries a logon scheduled task,
// then a user Run-key startup entry. Returns the method ID that succeeded.
func Install(opts Options) (string, error) {
	if err := installService(opts); err == nil {
		return MethodService, nil
	} else if !isAccessDenied(err) {
		return "", err
	}
	if err := installLogonTask(opts); err == nil {
		return MethodLogonTask, nil
	}
	if err := installRunKey(opts); err != nil {
		return "", fmt.Errorf("service/logon-task denied; startup entry failed: %w", err)
	}
	return MethodStartup, nil
}

func installService(opts Options) error {
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	if err := ensureDirs(opts); err != nil {
		return err
	}
	if err := copyFile(exe, BinaryPath()); err != nil {
		return fmt.Errorf("copy agent binary: %w", err)
	}
	_ = controlService(opts, "stop")
	_ = controlService(opts, "uninstall")
	if err := controlService(opts, "install"); err != nil {
		return fmt.Errorf("service install: %w", err)
	}
	if err := controlService(opts, "start"); err != nil {
		return fmt.Errorf("service start: %w", err)
	}
	return nil
}

func installLogonTask(opts Options) error {
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	if err := ensureDirs(opts); err != nil {
		return err
	}
	if err := copyFile(exe, BinaryPath()); err != nil {
		return fmt.Errorf("copy agent binary: %w", err)
	}

	args := []string{
		"-server", opts.Server,
		"-id-file", opts.IDFile,
		"-sleep", opts.Sleep,
	}
	if opts.Insecure {
		args = append(args, "-insecure")
	}
	tr := fmt.Sprintf(`"%s" %s`, BinaryPath(), strings.Join(quoteArgs(args), " "))
	cmd := exec.Command("schtasks.exe",
		"/Create", "/F",
		"/TN", taskName(),
		"/SC", "ONLOGON",
		"/RL", "LIMITED",
		"/TR", tr,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	// Start now so the operator does not wait for next logon.
	start := exec.Command("schtasks.exe", "/Run", "/TN", taskName())
	start.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = start.Run()
	return nil
}

func UninstallService() error {
	opts := Options{
		Server: "https://127.0.0.1:8443",
		IDFile: defaultIDFilePath(),
		Sleep:  "5s",
	}
	_ = controlService(opts, "stop")
	_ = controlService(opts, "uninstall")

	cmd := exec.Command("schtasks.exe", "/Delete", "/F", "/TN", taskName())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()

	_ = exec.Command("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", taskName(), "/f").Run()
	return nil
}

func installRunKey(opts Options) error {
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	if err := ensureDirs(opts); err != nil {
		return err
	}
	if err := copyFile(exe, BinaryPath()); err != nil {
		return fmt.Errorf("copy agent binary: %w", err)
	}

	args := []string{
		"-server", opts.Server,
		"-id-file", opts.IDFile,
		"-sleep", opts.Sleep,
	}
	if opts.Insecure {
		args = append(args, "-insecure")
	}
	value := fmt.Sprintf(`"%s" %s`, BinaryPath(), strings.Join(quoteArgs(args), " "))
	cmd := exec.Command("reg.exe", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", taskName(),
		"/t", "REG_SZ",
		"/d", value,
		"/f",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg add: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	startArgs := append([]string{}, args...)
	start := exec.Command(BinaryPath(), startArgs...)
	start.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := start.Start(); err != nil {
		return fmt.Errorf("start agent now: %w", err)
	}
	return nil
}

func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "access is denied") || strings.Contains(s, "access denied")
}

func quoteArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			out[i] = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		} else {
			out[i] = a
		}
	}
	return out
}
