package core

import (
	"sync"

	"github.com/digitizables/lattice/server/store"
)

// Event is a domain notification for operator streams.
type Event struct {
	Session *store.Agent
	Task    *store.Task
}

// Bus is an in-process pub/sub for operator events.
type Bus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

func newBus() *Bus {
	return &Bus{subs: make(map[chan Event]struct{})}
}

// Subscribe returns a buffered event channel and an unsubscribe func.
func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 16
	}
	ch := make(chan Event, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsub
}

func (b *Bus) publish(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Drop if a slow subscriber falls behind.
		}
	}
}
