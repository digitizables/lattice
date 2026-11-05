package listen

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// Channel is a pluggable agent-facing listener.
type Channel interface {
	Name() string
	Start(ctx context.Context, addr string) error
	Stop(ctx context.Context) error
}

// Info is a running (or known) listener snapshot for operators.
type Info struct {
	Name    string
	Kind    string
	Addr    string
	Version string
	TLS     bool
	Status  string // running
}

type running struct {
	ch   Channel
	info Info
}

// Manager starts and stops registered channels.
type Manager struct {
	mu       sync.Mutex
	channels map[string]Channel
	running  map[string]running
}

// NewManager returns an empty listener manager.
func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		running:  make(map[string]running),
	}
}

// Register adds a channel implementation by name.
func (m *Manager) Register(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.Name()] = ch
}

// Start runs a registered channel on addr and records operator-facing metadata.
func (m *Manager) Start(ctx context.Context, name, addr string, info Info) error {
	m.mu.Lock()
	ch, ok := m.channels[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown channel %q", name)
	}
	if _, busy := m.running[name]; busy {
		m.mu.Unlock()
		return fmt.Errorf("channel %q already running", name)
	}
	m.mu.Unlock()

	if err := ch.Start(ctx, addr); err != nil {
		return err
	}

	if info.Name == "" {
		info.Name = name
	}
	if info.Kind == "" {
		info.Kind = "channel"
	}
	if info.Addr == "" {
		info.Addr = addr
	}
	if info.Status == "" {
		info.Status = "running"
	}

	m.mu.Lock()
	m.running[name] = running{ch: ch, info: info}
	m.mu.Unlock()
	return nil
}

// List returns running listeners sorted by name.
func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.running))
	for _, r := range m.running {
		out = append(out, r.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Reset drops registered channel implementations. Call after StopAll before
// rediscovering manifests for a hot reload.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels = make(map[string]Channel)
	m.running = make(map[string]running)
}

// Stop halts a running channel.
func (m *Manager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	r, ok := m.running[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("channel %q is not running", name)
	}
	delete(m.running, name)
	m.mu.Unlock()
	return r.ch.Stop(ctx)
}

// StopAll stops every running channel.
func (m *Manager) StopAll(ctx context.Context) {
	m.mu.Lock()
	names := make([]string, 0, len(m.running))
	for name := range m.running {
		names = append(names, name)
	}
	m.mu.Unlock()
	for _, name := range names {
		_ = m.Stop(ctx, name)
	}
}

// HandlerFunc adapts a function to http.Handler.
type HandlerFunc func(http.ResponseWriter, *http.Request)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}
