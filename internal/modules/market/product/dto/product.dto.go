package dto

// ProductQueryParams is page/limit/q for GET /market/v1/products, same
// binding-tag convention as wallet's LedgerQueryParams: out-of-range page/
// limit is rejected with 400 before reaching the database. Q has no
// binding tag — an empty q means "no filter", not an error.
type ProductQueryParams struct {
	Page  int    `form:"page,default=1" binding:"min=1"`
	Limit int    `form:"limit,default=25" binding:"min=1,max=100"`
	Q     string `form:"q"`
}

// LapakSummaryResponse is the contract's LapakSummary schema, embedded in
// every ProductResponse so the UI can show the seller without a second call.
type LapakSummaryResponse struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Rating float64 `json:"rating"`
}

// ProductResponse is the contract's Product schema. PriceIDR is int64 so it
// always serializes as a JSON number, never a string.
type ProductResponse struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	PriceIDR    int64                `json:"price_idr"`
	ImageURL    string               `json:"image_url"`
	Lapak       LapakSummaryResponse `json:"lapak"`
}
