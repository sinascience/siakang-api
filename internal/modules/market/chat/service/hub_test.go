package service

import (
	"sync"
	"testing"
	"time"

	"siakang-api/internal/modules/market/chat/domain"
)

// The three ways an SSE fan-out breaks, one assertion each. Run with -race:
// this is the only concurrent code in the sprint.
func TestHub(t *testing.T) {
	h := NewHub()
	msg := domain.Message{ID: "m1", ThreadID: "t1", Body: "halo"}

	// 1. Everyone on the thread gets it, the sender's own stream included,
	//    and nobody on another thread does.
	a, b := h.Subscribe("t1"), h.Subscribe("t1")
	other := h.Subscribe("t2")
	h.Publish("t1", msg)

	for i, ch := range []chan domain.Message{a, b} {
		select {
		case got := <-ch:
			if got.ID != "m1" {
				t.Fatalf("subscriber %d got %q, want m1", i, got.ID)
			}
		default:
			t.Fatalf("subscriber %d received nothing", i)
		}
	}
	if len(other) != 0 {
		t.Fatal("another thread's subscriber received the message")
	}

	// 2. Unsubscribe actually removes — a hub that only adds leaks a channel
	//    per EventSource reconnect.
	if remaining := h.Unsubscribe("t1", a); remaining != 1 {
		t.Fatalf("remaining = %d, want 1", remaining)
	}
	if h.Unsubscribe("t1", b); h.Count("t1") != 0 {
		t.Fatalf("count = %d after unsubscribing both, want 0", h.Count("t1"))
	}
	h.Unsubscribe("t2", other)

	// 3. A subscriber that never reads must not stall the send path. Fill one
	//    past its buffer and publish from a second goroutine: blocking here
	//    would block POST /messages in production.
	stalled := h.Subscribe("t3")
	defer h.Unsubscribe("t3", stalled)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < subscriberBuffer*3; i++ {
			h.Publish("t3", msg)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a subscriber that stopped reading")
	}

	// 4. Concurrent subscribe/publish/unsubscribe, for the race detector.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := h.Subscribe("t4")
			h.Publish("t4", msg)
			<-ch
			h.Unsubscribe("t4", ch)
		}()
	}
	wg.Wait()
	if h.Count("t4") != 0 {
		t.Fatalf("count = %d after all subscribers left, want 0", h.Count("t4"))
	}
}
