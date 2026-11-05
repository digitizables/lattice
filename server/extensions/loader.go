package extensions

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/digitizables/lattice/server/core"
	"github.com/digitizables/lattice/server/listen"
)

// ChannelFactory builds a channel from shared teamserver deps.
type ChannelFactory func(deps ChannelDeps) (listen.Channel, error)

// ChannelDeps is what in-process channel extensions receive today.
type ChannelDeps struct {
	Core      *core.Core
	Log       *slog.Logger
	CertFile  string
	KeyFile   string
	Cleartext bool // when true, skip TLS (lab escape hatch)
}

// Loader discovers manifests and constructs registered extension factories.
type Loader struct {
	log      *slog.Logger
	channels map[string]ChannelFactory
}

// New returns a loader with no factories registered.
func New(log *slog.Logger) *Loader {
	if log == nil {
		log = slog.Default()
	}
	return &Loader{
		log:      log,
		channels: make(map[string]ChannelFactory),
	}
}

// RegisterChannel binds a factory to a manifest name.
func (l *Loader) RegisterChannel(name string, factory ChannelFactory) {
	l.channels[name] = factory
}

// Discover walks root for channel manifests (extensions/channels/*/manifest.yaml).
func (l *Loader) Discover(root string) ([]Manifest, error) {
	channelsDir := filepath.Join(root, "channels")
	entries, err := os.ReadDir(channelsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("extensions directory %q has no channels/", root)
		}
		return nil, err
	}

	var out []Manifest
	for _, ent := range entries {
		if !ent.IsDir() || ent.Name() == "_template" {
			continue
		}
		dir := filepath.Join(channelsDir, ent.Name())
		path := filepath.Join(dir, "manifest.yaml")
		m, err := loadManifest(path)
		if err != nil {
			if os.IsNotExist(err) {
				l.log.Warn("skip channel dir without manifest", "dir", dir)
				continue
			}
			return nil, err
		}
		if m.Kind != "channel" {
			return nil, fmt.Errorf("%s: kind %q must be channel", path, m.Kind)
		}
		m.Path = dir
		out = append(out, m)
	}
	return out, nil
}

// StartChannels registers and starts each enabled channel manifest.
// defaultAddr is used when a manifest does not set bind_addr.
func (l *Loader) StartChannels(
	ctx context.Context,
	mgr *listen.Manager,
	deps ChannelDeps,
	defaultAddr string,
	manifests []Manifest,
) ([]string, error) {
	var started []string
	for _, m := range manifests {
		if !m.IsEnabled() {
			l.log.Info("channel disabled", "name", m.Name, "path", m.Path)
			continue
		}
		factory, ok := l.channels[m.Name]
		if !ok {
			return started, fmt.Errorf("no in-process factory registered for channel %q (manifest at %s)", m.Name, m.Path)
		}
		ch, err := factory(deps)
		if err != nil {
			return started, fmt.Errorf("channel %q: %w", m.Name, err)
		}
		if ch.Name() != m.Name {
			return started, fmt.Errorf("channel factory name %q does not match manifest %q", ch.Name(), m.Name)
		}
		mgr.Register(ch)
		addr := m.BindAddr
		if addr == "" {
			addr = defaultAddr
		}
		info := listen.Info{
			Name:    m.Name,
			Kind:    "channel",
			Addr:    addr,
			Version: m.Version,
			TLS:     !deps.Cleartext,
			Status:  "running",
		}
		if err := mgr.Start(ctx, m.Name, addr, info); err != nil {
			return started, fmt.Errorf("start channel %q: %w", m.Name, err)
		}
		l.log.Info("channel started", "name", m.Name, "addr", addr, "version", m.Version)
		started = append(started, m.Name)
	}
	if len(started) == 0 {
		return nil, fmt.Errorf("no enabled channels started from manifests")
	}
	return started, nil
}
