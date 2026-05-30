package logs

import "sync"

// Event is a single lifecycle/log event emitted by a job.
type Event struct {
	Step    string         `json:"step"`
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// Bus is a per-job event broadcaster. It retains the full event history so a
// subscriber that connects late still receives every event, then live updates
// until the bus is closed.
type Bus struct {
	mu      sync.Mutex
	history []Event
	subs    map[chan Event]struct{}
	closed  bool
}

// NewBus creates an empty event bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[chan Event]struct{})}
}

// Publish records an event and fans it out to all current subscribers.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.history = append(b.history, e)
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// Slow consumer: drop rather than block the producer. The full
			// history is still available via the initial replay.
		}
	}
}

// Subscribe returns the event history followed by a channel of future events.
// The channel is closed when the bus is closed. Callers must invoke the
// returned cancel function to release resources.
func (b *Bus) Subscribe() (history []Event, ch <-chan Event, cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	hist := make([]Event, len(b.history))
	copy(hist, b.history)
	c := make(chan Event, 64)
	if b.closed {
		close(c)
		return hist, c, func() {}
	}
	b.subs[c] = struct{}{}
	return hist, c, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[c]; ok {
			delete(b.subs, c)
			close(c)
		}
	}
}

// Close marks the bus closed and closes all subscriber channels. Further
// Publish calls are ignored.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
}

// Closed reports whether the bus has been closed.
func (b *Bus) Closed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}
