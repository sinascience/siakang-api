package dto

import (
	"time"

	"siakang-api/internal/modules/market/order/domain"
)

// CreateOrderRequest is the POST /market/v1/orders body. Exactly one of
// ProductID / GigTierID must be present — a rule `binding` cannot express, so
// the handler checks it and reports 422 with the field keys.
//
// There is deliberately no price field: a client-sent price is ignored
// entirely. unit_price_idr and name are snapshotted from the product row at
// order time, so later catalog edits never rewrite order history.
type CreateOrderRequest struct {
	ProductID string `json:"product_id" binding:"omitempty,uuid"`

	// Quantity is a pointer so "absent" and "0" stay distinguishable. As a
	// plain int, omitempty would skip min=1 for an explicit quantity:0 and
	// the request would silently buy one — a small money surprise, but a
	// money surprise. Nil means absent and defaults to 1; 0 is rejected.
	Quantity *int `json:"quantity" binding:"omitempty,min=1"`

	// GigTierID is accepted so the contract's request shape round-trips and
	// so BE-07's flow B is a branch here rather than a rewrite. Phase 2
	// rejects it: there is no half-finished gig path behind this field.
	GigTierID string `json:"gig_tier_id" binding:"omitempty,uuid"`
}

// OrderIDParam validates {id} before it reaches SQL. A non-uuid id cannot name
// a row, so the handler answers 404 — same answer a third party gets, which
// keeps the endpoint from confirming what does or does not exist.
type OrderIDParam struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// ListQueryParams is page/limit/status for GET /market/v1/orders. page/limit
// carry the same min/max binding tags as every other paginated list in this
// codebase, so an out-of-range limit is rejected rather than silently clamped.
type ListQueryParams struct {
	Page   int    `form:"page,default=1" binding:"min=1"`
	Limit  int    `form:"limit,default=25" binding:"min=1,max=100"`
	Status string `form:"status" binding:"omitempty,oneof=pending_payment paid awaiting_confirmation completed cancelled"`
}

// OrderItemResponse is the contract's OrderItem. ProductID/GigTierID have no
// omitempty: the contract marks them nullable, not optional, so a nil pointer
// must serialize as a JSON null rather than vanish.
type OrderItemResponse struct {
	ID           string    `json:"id"`
	ProductID    *string   `json:"product_id"`
	GigTierID    *string   `json:"gig_tier_id"`
	Name         string    `json:"name"`
	UnitPriceIDR int64     `json:"unit_price_idr"`
	Quantity     int       `json:"quantity"`
	SubtotalIDR  int64     `json:"subtotal_idr"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// PaymentResponse is the contract's Payment.
type PaymentResponse struct {
	ID           string    `json:"id"`
	OrderID      string    `json:"order_id"`
	AmountIDR    int64     `json:"amount_idr"`
	OrderItemIDs []string  `json:"order_item_ids"`
	PaidAt       time.Time `json:"paid_at"`
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

// OrderResponse is the contract's Order. Every *_idr is int64 so it always
// serializes as a JSON number — never a string, never a float.
type OrderResponse struct {
	ID                string               `json:"id"`
	Source            string               `json:"source"`
	Status            string               `json:"status"`
	Customer          UserSummaryResponse  `json:"customer"`
	Lapak             LapakSummaryResponse `json:"lapak"`
	Items             []OrderItemResponse  `json:"items"`
	Payments          []PaymentResponse    `json:"payments"`
	TotalIDR          int64                `json:"total_idr"`
	PaidIDR           int64                `json:"paid_idr"`
	OutstandingIDR    int64                `json:"outstanding_idr"`
	BidID             *string              `json:"bid_id"`
	ChatThreadID      *string              `json:"chat_thread_id"`
	DeliveryStatus    string               `json:"delivery_status"`
	ConfirmDeadlineAt *time.Time           `json:"confirm_deadline_at"`
	AutoConfirmed     bool                 `json:"auto_confirmed"`
	CompletedAt       *time.Time           `json:"completed_at"`
	CreatedAt         time.Time            `json:"created_at"`
}

// PayResultResponse is the contract's PayResult. WalletBalanceIDR is the
// balance AFTER the charge, so FE need not refetch the wallet.
type PayResultResponse struct {
	Order            OrderResponse   `json:"order"`
	Payment          PaymentResponse `json:"payment"`
	WalletBalanceIDR int64           `json:"wallet_balance_idr"`
}

// NewOrderResponse maps a domain order onto the contract shape. The three
// money totals are computed here from the items, matching the schema's
// "derived, not stored" rule.
func NewOrderResponse(o *domain.Order) OrderResponse {
	items := make([]OrderItemResponse, 0, len(o.Items))
	for _, i := range o.Items {
		items = append(items, OrderItemResponse{
			ID:           i.ID,
			ProductID:    i.ProductID,
			GigTierID:    i.GigTierID,
			Name:         i.Name,
			UnitPriceIDR: i.UnitPriceIDR,
			Quantity:     i.Quantity,
			SubtotalIDR:  i.SubtotalIDR,
			Status:       i.Status,
			CreatedAt:    i.CreatedAt,
		})
	}

	payments := make([]PaymentResponse, 0, len(o.Payments))
	for _, p := range o.Payments {
		payments = append(payments, NewPaymentResponse(p))
	}

	return OrderResponse{
		ID:                o.ID,
		Source:            o.Source,
		Status:            o.Status,
		Customer:          UserSummaryResponse{ID: o.Customer.ID, FullName: o.Customer.FullName},
		Lapak:             LapakSummaryResponse{ID: o.Lapak.ID, Name: o.Lapak.Name, Rating: o.Lapak.Rating},
		Items:             items,
		Payments:          payments,
		TotalIDR:          o.TotalIDR(),
		PaidIDR:           o.PaidIDR(),
		OutstandingIDR:    o.OutstandingIDR(),
		BidID:             o.BidID,
		ChatThreadID:      o.ChatThreadID,
		DeliveryStatus:    o.DeliveryStatus,
		ConfirmDeadlineAt: o.ConfirmDeadlineAt,
		AutoConfirmed:     o.AutoConfirmed,
		CompletedAt:       o.CompletedAt,
		CreatedAt:         o.CreatedAt,
	}
}

// NewPaymentResponse maps one payment. OrderItemIDs is normalized to a
// non-nil slice so it serializes as [] rather than null.
func NewPaymentResponse(p domain.Payment) PaymentResponse {
	ids := p.OrderItemIDs
	if ids == nil {
		ids = []string{}
	}
	return PaymentResponse{
		ID:           p.ID,
		OrderID:      p.OrderID,
		AmountIDR:    p.AmountIDR,
		OrderItemIDs: ids,
		PaidAt:       p.PaidAt,
	}
}
