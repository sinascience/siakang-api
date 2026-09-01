package dto

import (
	"time"

	"siakang-api/internal/modules/market/chat/domain"
)

// ThreadIDParam validates {id} before it reaches SQL. A non-uuid id names no
// row, so the handler answers 404 — the same answer a non-participant gets,
// which keeps the endpoint from confirming what exists.
type ThreadIDParam struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// ListQueryParams is page/limit, with the contract's min/max as binding tags
// so an out-of-range limit is rejected rather than silently clamped.
type ListQueryParams struct {
	Page  int `form:"page,default=1" binding:"min=1"`
	Limit int `form:"limit,default=25" binding:"min=1,max=100"`
}

// SendMessageRequest is the POST body. `required` rejects "" but not "   ",
// and the schema's CHECK (LENGTH(TRIM(body)) > 0) would turn a whitespace-only
// body into a 500 — so the handler trims and reports 422 before the insert.
type SendMessageRequest struct {
	Body string `json:"body" binding:"required,max=2000"`
}

// MessageResponse is the contract's ChatMessage — used by both the REST
// endpoints and the SSE chat.message frame.
type MessageResponse struct {
	ID           string    `json:"id"`
	ThreadID     string    `json:"thread_id"`
	SenderUserID string    `json:"sender_user_id"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserSummaryResponse / LapakSummaryResponse are the contract's embedded
// participant summaries.
type UserSummaryResponse struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type LapakSummaryResponse struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Rating float64 `json:"rating"`
}

// ThreadResponse is the contract's ChatThread. LastMessage has no omitempty:
// it is nullable, so an empty thread must serialize `"last_message": null`
// rather than drop the key.
type ThreadResponse struct {
	ID          string               `json:"id"`
	OrderID     string               `json:"order_id"`
	Customer    UserSummaryResponse  `json:"customer"`
	Lapak       LapakSummaryResponse `json:"lapak"`
	LastMessage *MessageResponse     `json:"last_message"`
	CreatedAt   time.Time            `json:"created_at"`
}

// NewMessageResponse maps a domain message onto the contract shape.
func NewMessageResponse(m domain.Message) MessageResponse {
	return MessageResponse{
		ID:           m.ID,
		ThreadID:     m.ThreadID,
		SenderUserID: m.SenderUserID,
		Body:         m.Body,
		CreatedAt:    m.CreatedAt,
	}
}

// NewThreadResponse maps a domain thread onto the contract shape.
func NewThreadResponse(t *domain.Thread) ThreadResponse {
	var last *MessageResponse
	if t.LastMessage != nil {
		m := NewMessageResponse(*t.LastMessage)
		last = &m
	}

	return ThreadResponse{
		ID:      t.ID,
		OrderID: t.OrderID,
		Customer: UserSummaryResponse{
			ID:       t.Customer.ID,
			FullName: t.Customer.FullName,
		},
		Lapak: LapakSummaryResponse{
			ID:     t.Lapak.ID,
			Name:   t.Lapak.Name,
			Rating: t.Lapak.Rating,
		},
		LastMessage: last,
		CreatedAt:   t.CreatedAt,
	}
}
