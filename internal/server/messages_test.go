package server

import "testing"

func TestQueueEnqueueDrainAck(t *testing.T) {
	q := NewQueue()
	m1 := q.Enqueue("agent1", "hello")
	m2 := q.Enqueue("agent1", "world")
	q.Enqueue("agent2", "other")

	p := q.Pending("agent1")
	if len(p) != 2 || p[0].ID != m1.ID || p[1].ID != m2.ID {
		t.Fatalf("unexpected pending: %+v", p)
	}
	if !q.Ack(m1.ID) {
		t.Fatal("ack m1 failed")
	}
	if got := q.Pending("agent1"); len(got) != 1 || got[0].ID != m2.ID {
		t.Fatalf("after ack: %+v", got)
	}
	if q.Ack("nonexistent") {
		t.Fatal("ack of nonexistent should be false")
	}
}
