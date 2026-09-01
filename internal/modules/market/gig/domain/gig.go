package domain

// Gig is one row of market.gigs joined to its selling market.lapak_profiles
// row, with its tiers loaded. The catalog is platform-wide (product ruling
// 2026-09-02): no company_id, no owner filter, every signed-in user sees
// every gig.
type Gig struct {
	ID          string
	Title       string
	Description string
	ImageURL    string

	LapakID     string
	LapakName   string
	LapakRating float64

	// Tiers is price_idr ascending — the API's sort, not a stored column.
	Tiers []GigTier
}

// GigTier is one row of market.gig_tiers. GigID is populated — field
// contract amendment v1.0.3 — since nothing else in the API resolves a
// tier back to its gig.
type GigTier struct {
	ID          string
	GigID       string
	Name        string
	Description string
	PriceIDR    int64
}
