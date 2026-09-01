package dto

// GigQueryParams is page/limit/q for GET /market/v1/gigs, same
// binding-tag convention as product's ProductQueryParams: out-of-range
// page/limit is rejected with 400 before reaching the database. Q has no
// binding tag — an empty q means "no filter", not an error.
type GigQueryParams struct {
	Page  int    `form:"page,default=1" binding:"min=1"`
	Limit int    `form:"limit,default=25" binding:"min=1,max=100"`
	Q     string `form:"q"`
}

// LapakSummaryResponse is the contract's LapakSummary schema, embedded in
// every GigResponse so the UI can show the seller without a second call.
type LapakSummaryResponse struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Rating float64 `json:"rating"`
}

// GigTierResponse is the contract's GigTier schema. GigID is populated —
// field contract amendment v1.0.3 — since nothing else in the API resolves
// a tier back to its gig. PriceIDR is int64 so it always serializes as a
// JSON number, never a string.
type GigTierResponse struct {
	ID          string `json:"id"`
	GigID       string `json:"gig_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceIDR    int64  `json:"price_idr"`
}

// GigResponse is the contract's Gig schema. Tiers is ordered by price_idr
// ascending — FE renders the upsell ladder in that order.
type GigResponse struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	ImageURL    string               `json:"image_url"`
	Lapak       LapakSummaryResponse `json:"lapak"`
	Tiers       []GigTierResponse    `json:"tiers"`
}
