package server

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Event is a hook payload received from Claude Code. The body is kept as raw
// JSON so we never need to know the exact schema — we only project a few
// well-known fields for indexing.
type Event struct {
	ID         uint64          `json:"id"`
	ReceivedAt time.Time       `json:"received_at"`
	SessionID  string          `json:"session_id,omitempty"`
	Kind       string          `json:"kind,omitempty"`
	CWD        string          `json:"cwd,omitempty"`
	Workspace  string          `json:"workspace,omitempty"`
	Host       string          `json:"host,omitempty"`
	Label      string          `json:"label,omitempty"`
	Raw        json.RawMessage `json:"raw"`
}

// Bus is a fixed-size ring buffer plus SSE fan-out. Lock-light: writes take
// the mutex briefly; readers copy slices under the same mutex.
type Bus struct {
	mu       sync.RWMutex
	ring     []Event
	head     int
	size     int
	capacity int
	nextID   atomic.Uint64

	subsMu sync.Mutex
	subs   map[chan Event]struct{}
}

func NewBus(capacity int) *Bus {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Bus{
		ring:     make([]Event, capacity),
		capacity: capacity,
		subs:     map[chan Event]struct{}{},
	}
}

func (b *Bus) Publish(e Event) Event {
	e.ID = b.nextID.Add(1)
	if e.ReceivedAt.IsZero() {
		e.ReceivedAt = time.Now().UTC()
	}
	b.mu.Lock()
	b.ring[b.head] = e
	b.head = (b.head + 1) % b.capacity
	if b.size < b.capacity {
		b.size++
	}
	b.mu.Unlock()

	b.subsMu.Lock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// Slow subscriber: drop. The client will see a gap (increasing
			// IDs); they can reconcile via /events?since_id=.
		}
	}
	b.subsMu.Unlock()
	return e
}

// Snapshot returns events in insertion order, optionally filtered by
// sessionID and since (exclusive). If limit > 0, returns the most recent N.
func (b *Bus) Snapshot(sessionID string, sinceID uint64, limit int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Event, 0, b.size)
	start := (b.head - b.size + b.capacity) % b.capacity
	for i := 0; i < b.size; i++ {
		e := b.ring[(start+i)%b.capacity]
		if sessionID != "" && e.SessionID != sessionID {
			continue
		}
		if e.ID <= sinceID {
			continue
		}
		out = append(out, e)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// LatestBySession returns the most recent event per session_id.
func (b *Bus) LatestBySession() map[string]Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	latest := map[string]Event{}
	start := (b.head - b.size + b.capacity) % b.capacity
	for i := 0; i < b.size; i++ {
		e := b.ring[(start+i)%b.capacity]
		if e.SessionID == "" {
			continue
		}
		if prev, ok := latest[e.SessionID]; !ok || e.ID > prev.ID {
			latest[e.SessionID] = e
		}
	}
	return latest
}

func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.subsMu.Lock()
	b.subs[ch] = struct{}{}
	b.subsMu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.subsMu.Lock()
	delete(b.subs, ch)
	b.subsMu.Unlock()
	close(ch)
}
