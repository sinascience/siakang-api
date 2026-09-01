package domain

// LapakProfile mirrors a row of market.lapak_profiles for a single owner.
type LapakProfile struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Lat         float64
	Lng         float64
	Rating      float64
	IsAvailable bool
}
