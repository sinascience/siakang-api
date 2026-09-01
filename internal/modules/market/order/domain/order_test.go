package domain

import "testing"

// The three totals are what POST /pay charges and what FE renders as "you will
// pay X", so they are the one piece of this module worth pinning without a
// database. The mixed case is the flow-B shape: one item already covered by a
// payment, one still outstanding, on a single order.
func TestOrderTotals(t *testing.T) {
	tests := []struct {
		name                     string
		items                    []OrderItem
		total, paid, outstanding int64
	}{
		{
			name: "no items",
		},
		{
			name:  "single unpaid item",
			items: []OrderItem{{SubtotalIDR: 350000, Status: ItemStatusUnpaid}},
			total: 350000, paid: 0, outstanding: 350000,
		},
		{
			name:  "single paid item",
			items: []OrderItem{{SubtotalIDR: 350000, Status: ItemStatusPaid}},
			total: 350000, paid: 350000, outstanding: 0,
		},
		{
			name: "partly paid order — the upsell shape",
			items: []OrderItem{
				{SubtotalIDR: 10000, Status: ItemStatusPaid},
				{SubtotalIDR: 150000, Status: ItemStatusUnpaid},
			},
			total: 160000, paid: 10000, outstanding: 150000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := Order{Items: tt.items}
			if got := o.TotalIDR(); got != tt.total {
				t.Errorf("TotalIDR() = %d, want %d", got, tt.total)
			}
			if got := o.PaidIDR(); got != tt.paid {
				t.Errorf("PaidIDR() = %d, want %d", got, tt.paid)
			}
			if got := o.OutstandingIDR(); got != tt.outstanding {
				t.Errorf("OutstandingIDR() = %d, want %d", got, tt.outstanding)
			}
			// paid + outstanding == total is the invariant the pay path relies
			// on: an item is in exactly one of the two buckets.
			if tt.paid+tt.outstanding != tt.total {
				t.Errorf("paid+outstanding = %d, want total %d", tt.paid+tt.outstanding, tt.total)
			}
		})
	}
}
