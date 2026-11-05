package extensions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/digitizables/lattice/server/listen"
)

// Runtime owns discovery + start/stop so operators can reload channel
// manifests without restarting the whole teamserver process.
type Runtime struct {
	mu           sync.Mutex
	log          *slog.Logger
	loader       *Loader
	mgr          *listen.Manager
	deps         ChannelDeps
	extensionsDir string
	defaultAddr  string
}

// NewRuntime wires the loader and listener manager for later Reload calls.
func NewRuntime(log *slog.Logger, loader *Loader, mgr *listen.Manager, deps ChannelDeps, extensionsDir, defaultAddr string) *Runtime {
	if log == nil {
		log = slog.Default()
	}
	return &Runtime{
		log:           log,
		loader:        loader,
		mgr:           mgr,
		deps:          deps,
		extensionsDir: extensionsDir,
		defaultAddr:   defaultAddr,
	}
}

// Reload stops running channels, rediscovers manifests, and starts them again.
func (r *Runtime) Reload(ctx context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loader == nil || r.mgr == nil {
		return nil, fmt.Errorf("extension runtime not configured")
	}
	r.log.Info("reloading extensions", "dir", r.extensionsDir)
	r.mgr.StopAll(ctx)
	r.mgr.Reset()
	manifests, err := r.loader.Discover(r.extensionsDir)
	if err != nil {
		return nil, err
	}
	started, err := r.loader.StartChannels(ctx, r.mgr, r.deps, r.defaultAddr, manifests)
	if err != nil {
		return started, err
	}
	r.log.Info("extensions reloaded", "started", started)
	return started, nil
}
