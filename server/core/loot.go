package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentv1 "github.com/digitizables/lattice/gen/lattice/agent/v1"
	"github.com/digitizables/lattice/server/store"
)

// LootStore persists agent-uploaded files under a directory and Postgres metadata.
type LootStore struct {
	dir   string
	store *store.Store
}

// NewLootStore creates the loot directory.
func NewLootStore(dir string, st *store.Store) (*LootStore, error) {
	if dir == "" {
		dir = "data/loot"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &LootStore{dir: dir, store: st}, nil
}

// Dir returns the on-disk root.
func (l *LootStore) Dir() string { return l.dir }

// HandleLootUpload stores bytes from an agent.
func (c *Core) HandleLootUpload(ctx context.Context, in *agentv1.AgentLootUpload) (*agentv1.AgentLootUploadResponse, error) {
	if !agentTokenOK(c.agentToken, in.GetAgentToken()) {
		return nil, fmt.Errorf("invalid agent token")
	}
	if c.loot == nil {
		return nil, fmt.Errorf("loot store not configured")
	}
	if in.GetAgentId() == "" {
		return nil, fmt.Errorf("agent_id required")
	}
	data := in.GetData()
	if len(data) == 0 {
		return nil, fmt.Errorf("empty loot")
	}
	if len(data) > 32<<20 {
		return nil, fmt.Errorf("loot too large (max 32MiB)")
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	id := store.NewID()
	name := strings.TrimSpace(in.GetName())
	if name == "" {
		name = filepath.Base(strings.TrimSpace(in.GetRemotePath()))
	}
	if name == "" || name == "." || name == "/" {
		name = id
	}
	rel := filepath.Join(in.GetAgentId(), id+"_"+sanitizeName(name))
	full := filepath.Join(c.loot.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return nil, err
	}
	ct := in.GetContentType()
	if ct == "" {
		ct = "application/octet-stream"
	}
	if err := c.store.InsertLoot(ctx, store.Loot{
		ID:          id,
		AgentID:     in.GetAgentId(),
		TaskID:      in.GetTaskId(),
		Name:        name,
		RemotePath:  in.GetRemotePath(),
		ContentType: ct,
		SizeBytes:   uint64(len(data)),
		SHA256:      sha,
		StoragePath: rel,
	}); err != nil {
		_ = os.Remove(full)
		return nil, err
	}
	return &agentv1.AgentLootUploadResponse{Accepted: true, LootId: id}, nil
}

// HandleLootDownload returns staged loot bytes for an agent upload task.
func (c *Core) HandleLootDownload(ctx context.Context, in *agentv1.AgentLootDownload) (*agentv1.AgentLootDownloadResponse, error) {
	if !agentTokenOK(c.agentToken, in.GetAgentToken()) {
		return nil, fmt.Errorf("invalid agent token")
	}
	if c.loot == nil {
		return nil, fmt.Errorf("loot store not configured")
	}
	item, err := c.store.GetLoot(ctx, in.GetLootId())
	if err != nil {
		return &agentv1.AgentLootDownloadResponse{Found: false}, nil
	}
	data, err := os.ReadFile(filepath.Join(c.loot.dir, item.StoragePath))
	if err != nil {
		return nil, err
	}
	return &agentv1.AgentLootDownloadResponse{
		Found:       true,
		Name:        item.Name,
		Data:        data,
		ContentType: item.ContentType,
	}, nil
}

// ListLoot pages loot metadata.
func (c *Core) ListLoot(ctx context.Context, agentID string, pageSize int32, pageToken string) (store.LootPage, error) {
	return c.store.ListLootPage(ctx, agentID, pageSize, pageToken)
}

// GetLoot loads metadata and optionally file bytes.
func (c *Core) GetLoot(ctx context.Context, id string, includeData bool) (store.Loot, []byte, error) {
	item, err := c.store.GetLoot(ctx, id)
	if err != nil {
		return store.Loot{}, nil, err
	}
	if !includeData || c.loot == nil {
		return item, nil, nil
	}
	data, err := os.ReadFile(filepath.Join(c.loot.dir, item.StoragePath))
	if err != nil {
		return item, nil, err
	}
	return item, data, nil
}

// DeleteLoot removes metadata and file.
func (c *Core) DeleteLoot(ctx context.Context, id string) error {
	item, err := c.store.GetLoot(ctx, id)
	if err != nil {
		return err
	}
	if err := c.store.DeleteLoot(ctx, id); err != nil {
		return err
	}
	if c.loot != nil && item.StoragePath != "" {
		_ = os.Remove(filepath.Join(c.loot.dir, item.StoragePath))
	}
	return nil
}

// SetSessionNote updates the operator note.
func (c *Core) SetSessionNote(ctx context.Context, agentID, note string) (store.Agent, error) {
	if err := c.store.SetAgentNote(ctx, agentID, note); err != nil {
		return store.Agent{}, err
	}
	return c.store.GetAgent(ctx, agentID)
}

// SetAgentSleep queues sleep/jitter for the next check-in.
func (c *Core) SetAgentSleep(ctx context.Context, agentID string, sleepSeconds uint64, jitterPercent uint32) (store.Agent, error) {
	if err := c.store.SetAgentDesiredSleep(ctx, agentID, sleepSeconds, jitterPercent); err != nil {
		return store.Agent{}, err
	}
	return c.store.GetAgent(ctx, agentID)
}

func sanitizeName(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "file"
	}
	return out
}
