package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/digitizables/lattice/internal/version"
)

func (a *Agent) runUpdate(ctx context.Context) (int32, string, string) {
	if a.Transport == nil {
		return 1, "", "no transport"
	}
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	check, err := a.Transport.CheckUpdate(checkCtx, a.ID, runtime.GOOS, runtime.GOARCH, version.Version)
	if err != nil {
		return 1, "", "update check failed: " + err.Error()
	}
	if !check.GetAvailable() || check.GetUpdate() == nil {
		return 0, "already up to date (" + version.Version + ")", ""
	}
	info := check.GetUpdate()
	dlCtx, dlCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer dlCancel()
	dl, err := a.Transport.DownloadUpdate(dlCtx, a.ID, runtime.GOOS, runtime.GOARCH, info.GetVersion())
	if err != nil {
		return 1, "", "download failed: " + err.Error()
	}
	data := dl.GetData()
	meta := dl.GetUpdate()
	if meta == nil {
		meta = info
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, meta.GetSha256()) {
		return 1, "", fmt.Sprintf("sha256 mismatch: got %s want %s", got, meta.GetSha256())
	}
	if err := applyUpdate(data); err != nil {
		return 1, "", "apply failed: " + err.Error()
	}
	a.exitAfterTask = true
	msg := fmt.Sprintf("updated %s -> %s (%d bytes)\nrestarting agent process", version.Version, meta.GetVersion(), len(data))
	return 0, msg, ""
}

func writeTempUpdate(data []byte) (string, error) {
	f, err := os.CreateTemp("", "lattice-agent-update-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Chmod(0o755); err != nil {
		// Windows may ignore chmod; continue.
	}
	return f.Name(), nil
}
