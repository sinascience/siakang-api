// Package domain holds the chat entities. A thread has no participants of its
// own: customer and lapak are read through the thread's order, which is also
// where authorization comes from (market.chat_threads has only order_id).
package domain

import "time"

// UserSummary is the contract's UserSummary — the thread's customer.
type UserSummary struct {
	ID       string
	FullName string
}

// LapakSummary is the contract's LapakSummary — the thread's worker.
type LapakSummary struct {
	ID     string
	Name   string
	Rating float64
}

// Message is one row of market.chat_messages. It is also the unit the hub
// fans out: the SSE frame and the REST body are the same shape, so a client
// that renders history renders a live message with the same code.
type Message struct {
	ID           string
	ThreadID     string
	SenderUserID string
	Body         string
	CreatedAt    time.Time
}

// Thread is one row of market.chat_threads with its order's participants
// resolved. LastMessage is nil when nothing has been said yet — the contract
// marks it nullable, not optional.
type Thread struct {
	ID          string
	OrderID     string
	Customer    UserSummary
	Lapak       LapakSummary
	LastMessage *Message
	CreatedAt   time.Time
}
