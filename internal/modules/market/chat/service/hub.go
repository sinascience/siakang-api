package service

import (
	"sync"

	"siakang-api/internal/modules/market/chat/domain"
	"siakang-api/pkg/logger"
)

// subscriberBuffer is how far behind a stream may fall before the hub starts
// dropping frames for it. A browser reading an event stream consumes frames as
// fast as the socket delivers them, so being more than a handful behind means
// the client is wedged or gone, not merely slow.
const subscriberBuffer = 16

// Hub fans a thread's messages out to the SSE streams currently open on it.
//
// In-process on purpose. Sprint 1 runs a single instance, so a Redis pub/sub
// layer would be a moving part with no reader — Redis is in the stack for the
// permission cache, and putting a channel per thread on it buys nothing until
// there is a second instance to reach. When there is one, Publish and
// Subscribe are the only two functions that change; the handler never learns
// where a message came from.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan domain.Message]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[chan domain.Message]struct{})}
}

// Subscribe registers a stream on a thread and returns its channel. Every
// caller must pair it with Unsubscribe — see the handler's defer. A hub that
// only ever adds leaks a goroutine and a channel per reconnect, and
// EventSource reconnects on every network blip.
func (h *Hub) Subscribe(threadID string) chan domain.Message {
	ch := make(chan domain.Message, subscriberBuffer)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subs[threadID] == nil {
		h.subs[threadID] = make(map[chan domain.Message]struct{})
	}
	h.subs[threadID][ch] = struct{}{}
	return ch
}

// Unsubscribe removes a stream and returns how many remain on that thread.
// The channel is not closed: the subscriber is the only reader and it is the
// one calling this, so closing would buy nothing and add a double-close to
// worry about. The thread's map is deleted when it empties, so the outer map
// does not accumulate a key for every thread ever streamed.
func (h *Hub) Unsubscribe(threadID string, ch chan domain.Message) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subs[threadID], ch)
	remaining := len(h.subs[threadID])
	if remaining == 0 {
		delete(h.subs, threadID)
	}
	return remaining
}

// Publish delivers a message to every open stream on the thread, including
// the sender's own — the contract's echo guarantee, so FE renders from one
// path instead of reconciling an optimistic bubble against a server echo.
//
// The send is non-blocking. POST /messages calls this on the request path and
// must never stall because one subscriber stopped reading: a dropped frame is
// recoverable (EventSource reconnects and FE refetches history), a wedged
// write path is not. Holding the mutex is safe precisely because no branch
// here can block.
func (h *Hub) Publish(threadID string, msg domain.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs[threadID] {
		select {
		case ch <- msg:
		default:
			logger.Warn("Chat subscriber is not keeping up, dropping frame",
				logger.String("thread_id", threadID),
				logger.String("message_id", msg.ID),
			)
		}
	}
}

// Count is the number of streams open on a thread. Test and log support only.
func (h *Hub) Count(threadID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[threadID])
}
