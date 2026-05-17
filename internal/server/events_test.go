package server

import (
	"encoding/json"
	"testing"
)

func TestBusRingWraps(t *testing.T) {
	b := NewBus(3)
	for i := 0; i < 5; i++ {
		b.Publish(Event{SessionID: "s", Kind: "K", Raw: json.RawMessage(`{}`)})
	}
	snap := b.Snapshot("", 0, 0)
	if len(snap) != 3 {
		t.Fatalf("want 3 events, got %d", len(snap))
	}
	if snap[0].ID != 3 || snap[2].ID != 5 {
		t.Fatalf("expected IDs 3..5, got %d..%d", snap[0].ID, snap[2].ID)
	}
}

func TestBusFilterAndSince(t *testing.T) {
	b := NewBus(10)
	b.Publish(Event{SessionID: "a", Kind: "X", Raw: json.RawMessage(`{}`)})
	b.Publish(Event{SessionID: "b", Kind: "X", Raw: json.RawMessage(`{}`)})
	b.Publish(Event{SessionID: "a", Kind: "X", Raw: json.RawMessage(`{}`)})

	only := b.Snapshot("a", 0, 0)
	if len(only) != 2 {
		t.Fatalf("want 2 for session a, got %d", len(only))
	}
	since := b.Snapshot("", 2, 0)
	if len(since) != 1 || since[0].ID != 3 {
		t.Fatalf("since_id=2 should yield [id=3], got %+v", since)
	}
}

func TestBusLatestBySession(t *testing.T) {
	b := NewBus(10)
	b.Publish(Event{SessionID: "a", Kind: "first"})
	b.Publish(Event{SessionID: "b", Kind: "only"})
	b.Publish(Event{SessionID: "a", Kind: "last"})
	m := b.LatestBySession()
	if m["a"].Kind != "last" || m["b"].Kind != "only" {
		t.Fatalf("unexpected latest map: %+v", m)
	}
}

func TestBusSubscribeReceives(t *testing.T) {
	b := NewBus(10)
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)
	b.Publish(Event{SessionID: "x", Kind: "K"})
	select {
	case e := <-ch:
		if e.SessionID != "x" {
			t.Fatalf("got %+v", e)
		}
	default:
		t.Fatal("expected event on subscriber")
	}
}
