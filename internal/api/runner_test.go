package api

import (
	"sync"
	"testing"
)

func TestBrokerReplaysEarlyEventsAndClosesSafely(t *testing.T) {
	b := newBroker()
	b.publish(AgentEvent{Type: "text", Text: "Before subscription"})
	ch := b.subscribe()
	if event := <-ch; event.Text != "Before subscription" {
		t.Fatalf("lost initial stream: %+v", event)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); b.close() }()
	go func() { defer wg.Done(); b.unsubscribe(ch) }()
	wg.Wait()
	if _, open := <-ch; open {
		t.Fatal("subscription remained open")
	}
	// Late reconnects get the bounded history, then EOF, not an orphan channel.
	late := b.subscribe()
	if event := <-late; event.Text != "Before subscription" {
		t.Fatal("lost replay")
	}
	if _, open := <-late; open {
		t.Fatal("closed broker accepted a live subscriber")
	}
}

func TestBrokerReplayBound(t *testing.T) {
	b := newBroker()
	for i := 0; i < 1300; i++ {
		b.publish(AgentEvent{Type: "tool", At: int64(i)})
	}
	ch := b.subscribe()
	b.close()
	count := 0
	for range ch {
		count++
	}
	if count != 1024 {
		t.Fatalf("history is unbounded: %d", count)
	}
}
