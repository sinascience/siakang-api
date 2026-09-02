// Package domain holds the market bid entities — flow C, both modes.
//
// off_platform_risk, offer_count, accepted_offer_id and order_id are NOT
// columns: the schema's "derived, not stored" rule keeps them out of
// market.bids, so they are a method (risk) and read-side joins (the rest).
package domain

import "time"

// Bid modes and statuses, mirroring the schema's CHECK constraints. Each mode
// owns its own half of the status vocabulary — chk_bids_status_matches_mode
// rejects a manual bid sitting in `proposed` and vice versa.
const (
	ModeAuto   = "auto"
	ModeManual = "manual"

	// Automatic: proposed → customer_confirmed → accepted, or no_match.
	StatusProposed          = "proposed"
	StatusCustomerConfirmed = "customer_confirmed"
	StatusAccepted          = "accepted"
	StatusNoMatch           = "no_match"

	// Manual: open → awarded.
	StatusOpen    = "open"
	StatusAwarded = "awarded"

	// Either mode may end here. Nothing in sprint 1 writes it — there is no
	// cancel endpoint in the contract — but the vocabulary is the schema's.
	StatusCancelled = "cancelled"

	// Offer statuses. `awarded`, never `accepted`: `accepted` is a BidStatus
	// belonging to the automatic flow, and chk_bid_offers_status rejects it
	// here anyway (contract amendment v1.0.1).
	OfferStatusPending  = "pending"
	OfferStatusAwarded  = "awarded"
	OfferStatusRejected = "rejected"

	// Order sources a bid produces.
	OrderSourceAuto   = "bid_auto"
	OrderSourceManual = "bid_manual"

	// Ledger types this module writes. platform_fee is negative (money left
	// the customer), refund positive (it came back).
	LedgerTypePlatformFee = "platform_fee"
	LedgerTypeRefund      = "refund"

	// market.config keys. Read as rows, never hard-coded: QA edits them to
	// prove the fees are configurable.
	ConfigKeyAutoFee   = "bid_auto_fee_idr"
	ConfigKeyManualFee = "bid_manual_fee_idr"
)

// Category is the contract's BidCategory.
type Category struct {
	ID   string
	Name string
	Slug string
}

// UserSummary is the contract's UserSummary — the bid's customer.
type UserSummary struct {
	ID       string
	FullName string
}

// LapakSummary is the contract's LapakSummary — a matched or offering worker.
type LapakSummary struct {
	ID     string
	Name   string
	Rating float64
}

// Bid is one row of market.bids with its read-side derivations attached.
//
// Lat/Lng are pointers because they are NULL for a manual bid: that bid has no
// matching origin, and 0,0 is a real coordinate in the Gulf of Guinea, not an
// "absent".
type Bid struct {
	ID                string
	Mode              string
	Status            string
	Category          Category
	Customer          UserSummary
	Title             string
	Description       string
	BudgetIDR         int64
	Lat               *float64
	Lng               *float64
	FeePaidIDR        int64
	MatchedLapak      *LapakSummary
	MatchedDistanceKM *float64
	OfferCount        int
	AcceptedOfferID   *string
	OrderID           *string
	CreatedAt         time.Time
}

// OffPlatformRisk is criterion BE-09.3b and the exact field FE renders
// goal.md criterion 5's untracked-transaction warning from.
//
// True while a manual bid is still `open` — no on-platform agreement has been
// made, so the customer could still be talked into an untracked cash deal.
// False once `awarded`, and false for every automatic bid: those are matched
// and priced by the platform from the moment they are created.
func (b *Bid) OffPlatformRisk() bool {
	return b.Mode == ModeManual && b.Status == StatusOpen
}

// Offer is one row of market.bid_offers with its lapak summary loaded.
type Offer struct {
	ID        string
	BidID     string
	Lapak     LapakSummary
	AmountIDR int64
	Message   string
	Status    string
	CreatedAt time.Time
}

// Match is what the haversine search returns: the nearest available lapak in
// the bid's category, and how far away it is.
type Match struct {
	Lapak      LapakSummary
	DistanceKM float64
}
