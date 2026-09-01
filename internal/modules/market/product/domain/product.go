package domain

// Product is one row of market.products joined to its selling
// market.lapak_profiles row. The catalog is platform-wide (product ruling
// 2026-09-02): no company_id, no owner filter, every signed-in user sees
// every product.
type Product struct {
	ID          string
	Title       string
	Description string
	PriceIDR    int64
	ImageURL    string

	LapakID     string
	LapakName   string
	LapakRating float64
}
