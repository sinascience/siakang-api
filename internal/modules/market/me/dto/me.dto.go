package dto

// LapakProfileResponse is the contract's LapakProfile schema.
type LapakProfileResponse struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Rating      float64 `json:"rating"`
	IsAvailable bool    `json:"is_available"`
}

// MarketMeResponse is the contract's MarketMe schema. Lapak is a pointer
// (no omitempty) so a customer serializes as `"lapak": null` rather than
// dropping the key — the contract requires the field to always be present.
type MarketMeResponse struct {
	Lapak *LapakProfileResponse `json:"lapak"`
}
