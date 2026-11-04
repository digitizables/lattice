package extensions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InstallLocalChannel copies a channel package directory into
// extensionsDir/channels/<name>. The source must contain manifest.yaml.
// Remote GitHub install is not implemented yet; use a local checkout or archive extract.
func InstallLocalChannel(extensionsDir, srcDir string) (string, error) {
	srcDir = filepath.Clean(srcDir)
	manifestPath := filepath.Join(srcDir, "manifest.yaml")
	m, err := loadManifest(manifestPath)
	if err != nil {
		return "", err
	}
	if m.Kind != "channel" {
		return "", fmt.Errorf("manifest kind %q is not channel", m.Kind)
	}
	name := strings.TrimSpace(m.Name)
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid channel name %q", m.Name)
	}

	dst := filepath.Join(extensionsDir, "channels", name)
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return "", err
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return "", err
	}
	if absSrc == absDst {
		return absDst, nil // already installed in place
	}
	if strings.HasPrefix(absDst+string(os.PathSeparator), absSrc+string(os.PathSeparator)) ||
		strings.HasPrefix(absSrc+string(os.PathSeparator), absDst+string(os.PathSeparator)) {
		return "", fmt.Errorf("source and destination overlap: %s vs %s", absSrc, absDst)
	}
	if err := os.RemoveAll(dst); err != nil {
		return "", fmt.Errorf("clear destination: %w", err)
	}
	if err := copyDir(srcDir, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
