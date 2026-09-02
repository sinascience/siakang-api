package dto

import (
	"time"

	"siakang-api/internal/modules/market/bid/domain"
)

// CreateBidRequest is the POST /market/v1/bids body.
//
// budget_idr is required for BOTH modes (contract amendment during v1 review):
// manual bids anchor their offers to it, and automatic bids price the tracked
// order /accept creates from it. Leaving it optional for auto would let an
// accepted bid produce an order with no price, which the customer then cannot
// pay.
//
// The per-mode requirements — lat/lng for auto, title for manual — are checked
// in the handler, because `binding` has no tag for "required when another
// field has this value" and reporting them per field is what a 422 is for.
type CreateBidRequest struct {
	Mode        string   `json:"mode" binding:"required,oneof=auto manual"`
	CategoryID  string   `json:"category_id" binding:"required,uuid"`
	Title       string   `json:"title" binding:"omitempty,max=200"`
	Description string   `json:"description" binding:"omitempty,max=2000"`
	BudgetIDR   int64    `json:"budget_idr" binding:"required,min=1"`
	Lat         *float64 `json:"lat" binding:"omitempty,min=-90,max=90"`
	Lng         *float64 `json:"lng" binding:"omitempty,min=-180,max=180"`
}

// CreateOfferRequest is the POST /market/v1/bids/{id}/offers body. A lapak
// posting again replaces its own amount and message rather than stacking a
// second row, so there is nothing here to identify an existing offer with:
// the lapak IS the identity, resolved from the JWT.
type CreateOfferRequest struct {
	AmountIDR int64  `json:"amount_idr" binding:"required,min=1"`
	Message   string `json:"message" binding:"omitempty,max=500"`
}

// BidIDParam validates {id} before it reaches SQL. A non-uuid id cannot name a
// row, so the handler answers 404 — the same answer a stranger gets, which
// keeps the endpoint from confirming what does or does not exist, and keeps a
// malformed id off the 500 path.
type BidIDParam struct {
	ID string `uri:"id" binding:"required,uuid"`
}

// AwardParam is the two-segment path of /bids/{id}/offers/{offer_id}/award.
// Both are validated together, so a malformed offer id 404s exactly like a
// malformed bid id.
type AwardParam struct {
	ID      string `uri:"id" binding:"required,uuid"`
	OfferID string `uri:"offer_id" binding:"required,uuid"`
}

// ListQueryParams is page/limit/mode/status for GET /market/v1/bids. The
// min/max binding tags match every other paginated list in this codebase, so
// an out-of-range limit is rejected rather than silently clamped.
type ListQueryParams struct {
	Page   int    `form:"page,default=1" binding:"min=1"`
	Limit  int    `form:"limit,default=25" binding:"min=1,max=100"`
	Mode   string `form:"mode" binding:"omitempty,oneof=auto manual"`
	Status string `form:"status" binding:"omitempty,oneof=proposed customer_confirmed accepted no_match open awarded cancelled"`
}

// CategoryResponse is the contract's BidCategory.
type CategoryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// UserSummaryResponse / LapakSummaryResponse are the contract's embedded
// participant summaries, identical in shape to the order module's.
type UserSummaryResponse struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type LapakSummaryResponse struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Rating float64 `json:"rating"`
}

// BidResponse is the contract's Bid. Every nullable field is a pointer with no
// omitempty: the contract marks them nullable, not optional, so an absent
// value must serialize as JSON null rather than vanish from the object.
type BidResponse struct {
	ID                string                `json:"id"`
	Mode              string                `json:"mode"`
	Status            string                `json:"status"`
	Category          CategoryResponse      `json:"category"`
	Customer          UserSummaryResponse   `json:"customer"`
	Title             string                `json:"title"`
	Description       string                `json:"description"`
	BudgetIDR         int64                 `json:"budget_idr"`
	Lat               *float64              `json:"lat"`
	Lng               *float64              `json:"lng"`
	FeePaidIDR        int64                 `json:"fee_paid_idr"`
	MatchedLapak      *LapakSummaryResponse `json:"matched_lapak"`
	MatchedDistanceKM *float64              `json:"matched_distance_km"`
	OfferCount        int                   `json:"offer_count"`
	AcceptedOfferID   *string               `json:"accepted_offer_id"`
	OrderID           *string               `json:"order_id"`
	OffPlatformRisk   bool                  `json:"off_platform_risk"`
	CreatedAt         time.Time             `json:"created_at"`
}

// OfferResponse is the contract's BidOffer.
type OfferResponse struct {
	ID        string               `json:"id"`
	BidID     string               `json:"bid_id"`
	Lapak     LapakSummaryResponse `json:"lapak"`
	AmountIDR int64                `json:"amount_idr"`
	Message   string               `json:"message"`
	Status    string               `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
}

// NewBidResponse maps a domain bid onto the contract shape. off_platform_risk
// is computed here from mode and status — it is derived, not stored, so this
// is the only place it can come from.
func NewBidResponse(b *domain.Bid) BidResponse {
	var matched *LapakSummaryResponse
	if b.MatchedLapak != nil {
		matched = &LapakSummaryResponse{
			ID:     b.MatchedLapak.ID,
			Name:   b.MatchedLapak.Name,
			Rating: b.MatchedLapak.Rating,
		}
	}

	return BidResponse{
		ID:                b.ID,
		Mode:              b.Mode,
		Status:            b.Status,
		Category:          CategoryResponse{ID: b.Category.ID, Name: b.Category.Name, Slug: b.Category.Slug},
		Customer:          UserSummaryResponse{ID: b.Customer.ID, FullName: b.Customer.FullName},
		Title:             b.Title,
		Description:       b.Description,
		BudgetIDR:         b.BudgetIDR,
		Lat:               b.Lat,
		Lng:               b.Lng,
		FeePaidIDR:        b.FeePaidIDR,
		MatchedLapak:      matched,
		MatchedDistanceKM: b.MatchedDistanceKM,
		OfferCount:        b.OfferCount,
		AcceptedOfferID:   b.AcceptedOfferID,
		OrderID:           b.OrderID,
		OffPlatformRisk:   b.OffPlatformRisk(),
		CreatedAt:         b.CreatedAt,
	}
}

// NewOfferResponse maps one offer.
func NewOfferResponse(o *domain.Offer) OfferResponse {
	return OfferResponse{
		ID:        o.ID,
		BidID:     o.BidID,
		Lapak:     LapakSummaryResponse{ID: o.Lapak.ID, Name: o.Lapak.Name, Rating: o.Lapak.Rating},
		AmountIDR: o.AmountIDR,
		Message:   o.Message,
		Status:    o.Status,
		CreatedAt: o.CreatedAt,
	}
}

// NewCategoryResponse maps one category for GET /market/v1/bid-categories.
func NewCategoryResponse(c *domain.Category) CategoryResponse {
	return CategoryResponse{ID: c.ID, Name: c.Name, Slug: c.Slug}
}
