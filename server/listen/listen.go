package listen

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// Channel is a pluggable agent-facing listener.
type Channel interface {
	Name() string
	Start(ctx context.Context, addr string) error
	Stop(ctx context.Context) error
}

// Manager starts and stops registered channels.
type Manager struct {
	mu       sync.Mutex
	channels map[string]Channel
	running  map[string]Channel
}

// NewManager returns an empty listener manager.
func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		running:  make(map[string]Channel),
	}
}

// Register adds a channel implementation by name.
func (m *Manager) Register(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.Name()] = ch
}

// Start runs a registered channel on addr.
func (m *Manager) Start(ctx context.Context, name, addr string) error {
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

	m.mu.Lock()
	m.running[name] = ch
	m.mu.Unlock()
	return nil
}

// Stop halts a running channel.
func (m *Manager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	ch, ok := m.running[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("channel %q is not running", name)
	}
	delete(m.running, name)
	m.mu.Unlock()
	return ch.Stop(ctx)
}

// HandlerFunc adapts a function to http.Handler.
type HandlerFunc func(http.ResponseWriter, *http.Request)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}
