package dto

import "time"

// WalletResponse is the GET /market/v1/wallet body — contract's Wallet
// schema. BalanceIDR is int64 so it always serializes as a JSON number,
// never a string.
type WalletResponse struct {
	UserID     string `json:"user_id"`
	BalanceIDR int64  `json:"balance_idr"`
}

// LedgerEntryResponse is one row of GET /market/v1/wallet/ledger — contract's
// LedgerEntry schema. OrderID/BidID have no omitempty: the contract marks
// them nullable, not optional, so a nil *string must still serialize as the
// key with a JSON null, not be dropped.
type LedgerEntryResponse struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	AmountIDR       int64     `json:"amount_idr"`
	BalanceAfterIDR int64     `json:"balance_after_idr"`
	OrderID         *string   `json:"order_id"`
	BidID           *string   `json:"bid_id"`
	Note            string    `json:"note"`
	CreatedAt       time.Time `json:"created_at"`
}

// LedgerQueryParams is page/limit for the ledger list, matching every other
// paginated list in this codebase: default+min/max enforced via binding
// tags, so an out-of-range limit is rejected (400) rather than silently
// clamped — same convention as branch/company/user list endpoints.
type LedgerQueryParams struct {
	Page  int `form:"page,default=1" binding:"min=1"`
	Limit int `form:"limit,default=25" binding:"min=1,max=100"`
}
