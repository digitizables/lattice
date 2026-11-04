package install

import (
	"io"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
)

// Options configure how the agent service is installed and run.
type Options struct {
	Server   string
	IDFile   string
	Insecure bool
	Sleep    string // duration string, e.g. "5s"
}

// Program runs the agent under an OS service manager.
type Program struct {
	Run func()
}

func (p *Program) Start(s service.Service) error {
	go p.Run()
	return nil
}

func (p *Program) Stop(s service.Service) error {
	return nil
}

func controlService(opts Options, action string) error {
	prog := &Program{Run: func() {}}
	svc, err := NewService(opts, prog)
	if err != nil {
		return err
	}
	return service.Control(svc, action)
}

// NewService builds a service handle for install/uninstall/run.
func NewService(opts Options, prog *Program) (service.Service, error) {
	args := []string{
		"-server", opts.Server,
		"-id-file", opts.IDFile,
		"-sleep", opts.Sleep,
	}
	if opts.Insecure {
		args = append(args, "-insecure")
	}

	exe := BinaryPath()
	if _, err := os.Stat(exe); err != nil {
		self, err2 := os.Executable()
		if err2 == nil {
			self, _ = filepath.EvalSymlinks(self)
			exe = self
		}
	}

	cfg := &service.Config{
		Name:        ServiceName(),
		DisplayName: ServiceDisplayName(),
		Description: ServiceDescription(),
		Executable:  exe,
		Arguments:   args,
		Option: service.KeyValue{
			"Restart": "always",
		},
	}
	return service.New(prog, cfg)
}

func osExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func ensureDirs(opts Options) error {
	if err := os.MkdirAll(InstallDir(), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(opts.IDFile), 0o755)
}

func defaultIDFilePath() string {
	return filepath.Join(ConfigDir(), "agent.id")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
}
