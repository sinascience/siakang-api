// Package domain holds the market order entities. total_idr, paid_idr and
// outstanding_idr are deliberately NOT fields: the schema does not store them
// (a stored copy is a copy that can drift from the items that define it), so
// they are methods computed from Items.
package domain

import "time"

// Order statuses and sources, mirroring the schema's CHECK constraints.
const (
	StatusPendingPayment       = "pending_payment"
	StatusPaid                 = "paid"
	StatusAwaitingConfirmation = "awaiting_confirmation"
	StatusCompleted            = "completed"
	StatusCancelled            = "cancelled"

	ItemStatusUnpaid = "unpaid"
	ItemStatusPaid   = "paid"

	SourceProduct = "product"
	SourceGig     = "gig"

	LedgerTypeOrderPayment = "order_payment"
	// LedgerTypePayout is the positive counterpart: the lapak being paid when
	// the order completes, whether the customer confirmed or the sweeper did.
	LedgerTypePayout = "payout"
)

// OrderStatuses is every status in contract order, used to seed meta.counts so
// a status with no orders still reports 0 rather than going missing from the
// object and leaving an FE tab badge blank.
var OrderStatuses = []string{
	StatusPendingPayment,
	StatusPaid,
	StatusAwaitingConfirmation,
	StatusCompleted,
	StatusCancelled,
}

// UserSummary is the contract's UserSummary — the order's customer.
type UserSummary struct {
	ID       string
	FullName string
}

// LapakSummary is the contract's LapakSummary — the order's worker.
type LapakSummary struct {
	ID     string
	Name   string
	Rating float64
}

// OrderItem is one row of market.order_items. SubtotalIDR is read from the
// generated column rather than multiplied here, so the API and the database
// cannot disagree about it.
type OrderItem struct {
	ID        string
	OrderID   string
	ProductID *string
	GigTierID *string
	// GigID is the tier's parent gig (contract amendment v1.0.3). Non-nil
	// exactly when GigTierID is, so a client holding an order can offer the
	// gig's other tiers without scanning the catalogue.
	GigID        *string
	Name         string
	UnitPriceIDR int64
	Quantity     int
	SubtotalIDR  int64
	Status       string
	CreatedAt    time.Time
}

// Payment is one row of market.payments plus the items that charge covered.
// OrderItemIDs comes from market.order_items.payment_id read from the payment
// side — the schema has a real foreign key, not a UUID[] column.
type Payment struct {
	ID           string
	OrderID      string
	AmountIDR    int64
	OrderItemIDs []string
	PaidAt       time.Time
}

// Order is one row of market.orders with its items and payments loaded.
type Order struct {
	ID                string
	Source            string
	Status            string
	Customer          UserSummary
	Lapak             LapakSummary
	Items             []OrderItem
	Payments          []Payment
	BidID             *string
	ChatThreadID      *string
	DeliveryStatus    string
	ConfirmDeadlineAt *time.Time
	AutoConfirmed     bool
	CompletedAt       *time.Time
	CreatedAt         time.Time
}

// TotalIDR is the sum of every item.
func (o *Order) TotalIDR() int64 { return o.sum(func(OrderItem) bool { return true }) }

// PaidIDR is the sum of items already covered by a payment.
func (o *Order) PaidIDR() int64 {
	return o.sum(func(i OrderItem) bool { return i.Status == ItemStatusPaid })
}

// OutstandingIDR is the sum of unpaid items — exactly what the next
// POST /orders/{id}/pay will charge.
func (o *Order) OutstandingIDR() int64 {
	return o.sum(func(i OrderItem) bool { return i.Status == ItemStatusUnpaid })
}

func (o *Order) sum(keep func(OrderItem) bool) int64 {
	var total int64
	for _, i := range o.Items {
		if keep(i) {
			total += i.SubtotalIDR
		}
	}
	return total
}

// Product is the catalog row an order item is priced from. Only the fields the
// order path snapshots — the full product is BE-04's.
type Product struct {
	ID       string
	LapakID  string
	Title    string
	PriceIDR int64
}

// GigTier is Product's flow-B counterpart: the catalog row a gig order item is
// priced from. GigID is carried because the upsell is defined as "another tier
// of the SAME gig", so adding an item has to compare gigs, and LapakID is
// resolved through the tier's gig — a tier does not name a lapak itself.
//
// The full gig catalog is BE-07A's; these are only the fields the order path
// snapshots.
type GigTier struct {
	ID       string
	GigID    string
	LapakID  string
	Name     string
	PriceIDR int64
}
