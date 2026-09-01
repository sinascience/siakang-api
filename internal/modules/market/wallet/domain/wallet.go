package domain

import "time"

// Wallet is one row of market.wallets — a user's simulated balance.
type Wallet struct {
	UserID     string
	BalanceIDR int64
}

// LedgerEntry is one row of market.ledger_entries. AmountIDR is signed:
// negative means money left this wallet. OrderID/BidID are nullable — a
// topup is about neither.
type LedgerEntry struct {
	ID              string
	Type            string
	AmountIDR       int64
	BalanceAfterIDR int64
	OrderID         *string
	BidID           *string
	Note            string
	CreatedAt       time.Time
}
