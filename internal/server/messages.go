package server

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Message is a unit of work queued for delivery to an agent. Delivery is the
// consumer's job; the queue is delivery-agnostic.
type Message struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	Delivered bool      `json:"delivered"`
}

type Queue struct {
	mu sync.Mutex
	// per-agent FIFO of pending messages
	pending map[string][]*Message
	// id index for ack
	byID map[string]*Message
}

func NewQueue() *Queue {
	return &Queue{
		pending: map[string][]*Message{},
		byID:    map[string]*Message{},
	}
}

func newID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (q *Queue) Enqueue(agentID, body string) *Message {
	m := &Message{
		ID:        newID(),
		AgentID:   agentID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}
	q.mu.Lock()
	q.pending[agentID] = append(q.pending[agentID], m)
	q.byID[m.ID] = m
	q.mu.Unlock()
	return m
}

// Pending returns a snapshot of undelivered messages for an agent.
func (q *Queue) Pending(agentID string) []*Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	src := q.pending[agentID]
	out := make([]*Message, 0, len(src))
	for _, m := range src {
		if !m.Delivered {
			out = append(out, m)
		}
	}
	return out
}

// Ack marks a message delivered and removes it from the pending FIFO.
func (q *Queue) Ack(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	m, ok := q.byID[id]
	if !ok {
		return false
	}
	m.Delivered = true
	rest := q.pending[m.AgentID][:0]
	for _, x := range q.pending[m.AgentID] {
		if x.ID != id {
			rest = append(rest, x)
		}
	}
	q.pending[m.AgentID] = rest
	return true
}
