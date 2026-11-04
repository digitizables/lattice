//go:build unix

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Install prefers a systemd user unit, then an @reboot cron entry.
// Returns the method ID that succeeded.
func Install(opts Options) (string, error) {
	exe, err := osExecutable()
	if err != nil {
		return "", err
	}
	if err := ensureDirs(opts); err != nil {
		return "", err
	}
	if err := copyFile(exe, BinaryPath()); err != nil {
		return "", fmt.Errorf("copy agent binary: %w", err)
	}

	if err := installSystemdUser(opts); err == nil {
		return MethodLogonTask, nil
	}
	if err := installCronReboot(opts); err != nil {
		return "", err
	}
	return MethodStartup, nil
}

func installSystemdUser(opts Options) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return err
	}
	args := []string{BinaryPath(), "-server", opts.Server, "-id-file", opts.IDFile, "-sleep", opts.Sleep}
	if opts.Insecure {
		args = append(args, "-insecure")
	}
	unit := fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target

[Service]
ExecStart=%s
Restart=always
RestartSec=3

[Install]
WantedBy=default.target
`, ServiceDescription(), strings.Join(quoteUnix(args), " "))

	unitPath := filepath.Join(unitDir, ServiceName()+".service")
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", "--now", ServiceName()+".service").Run(); err != nil {
		return err
	}
	return nil
}

func installCronReboot(opts Options) error {
	line := fmt.Sprintf("@reboot %s -server %s -id-file %s -sleep %s",
		BinaryPath(), opts.Server, opts.IDFile, opts.Sleep)
	if opts.Insecure {
		line += " -insecure"
	}
	line += " # " + ServiceName()
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("(crontab -l 2>/dev/null | grep -v %q; echo %q) | crontab -", ServiceName(), line))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	// Start now.
	startArgs := []string{"-server", opts.Server, "-id-file", opts.IDFile, "-sleep", opts.Sleep}
	if opts.Insecure {
		startArgs = append(startArgs, "-insecure")
	}
	c := exec.Command(BinaryPath(), startArgs...)
	return c.Start()
}

func UninstallService() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", ServiceName()+".service").Run()
	home, err := os.UserHomeDir()
	if err == nil {
		_ = os.Remove(filepath.Join(home, ".config", "systemd", "user", ServiceName()+".service"))
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	_ = exec.Command("/bin/sh", "-c", fmt.Sprintf("crontab -l 2>/dev/null | grep -v %q | crontab -", ServiceName())).Run()
	opts := Options{Server: "https://127.0.0.1:8443", IDFile: defaultIDFilePath(), Sleep: "5s"}
	_ = controlService(opts, "stop")
	_ = controlService(opts, "uninstall")
	return nil
}

func quoteUnix(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return out
}
