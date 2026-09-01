package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/order/domain"
	"siakang-api/pkg/logger"
)

var (
	// ErrOrderNotFound means "no such order, OR the caller is not a
	// participant in it". The two are deliberately indistinguishable: every
	// read below carries the ownership test in its WHERE clause, so a third
	// party gets the same empty result as a bad id and the handler answers
	// 404. A 403 would confirm the order exists, which is itself a leak.
	ErrOrderNotFound = errors.New("order not found")

	// ErrProductNotFound covers a missing, soft-deleted, or dead-lapak
	// product.
	ErrProductNotFound = errors.New("product not found")

	// ErrGigTierNotFound is its flow-B twin: a missing, soft-deleted, or
	// dead-gig/dead-lapak tier.
	ErrGigTierNotFound = errors.New("gig tier not found")

	// ErrOrderNotPaid is /complete's 409: the order was not in status `paid`
	// when the UPDATE ran, so no countdown was started.
	ErrOrderNotPaid = errors.New("order is not in status paid")
)

// DB is the subset of pgx that both *pgxpool.Pool and pgx.Tx satisfy. Every
// method here takes one, so the same query runs on the pool for a plain read
// and on the transaction for the money path — without a second set of tx-only
// methods that could drift from these.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Pool exposes the pool so the service can Begin a transaction. Reads that
// need no transaction pass it straight back in as a DB.
func (r *Repository) Pool() *pgxpool.Pool { return r.db }

// =====================================================================
// Persona
// =====================================================================

// LapakIDForUser returns the caller's lapak profile id, or "" when the caller
// is a customer. One query answers two questions: whether creating an order is
// allowed (lapaks may not), and which orders the caller's list is scoped to.
func (r *Repository) LapakIDForUser(ctx context.Context, db DB, userID string) (string, error) {
	const query = `SELECT id FROM market.lapak_profiles WHERE user_id = $1 AND deleted_at IS NULL`

	var id string
	err := db.QueryRow(ctx, query, userID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		logger.Error("Failed to resolve lapak profile", logger.Err(err))
		return "", err
	}
	return id, nil
}

// =====================================================================
// Create
// =====================================================================

// FindProduct loads the catalog row an order item is priced from. This is the
// server-side price lookup: nothing the client sends reaches unit_price_idr.
//
// The join onto lapak_profiles is not decoration — an order must name a live
// lapak, and refusing here is cheaper than an order whose lapak summary is
// missing on every subsequent read.
func (r *Repository) FindProduct(ctx context.Context, db DB, productID string) (*domain.Product, error) {
	const query = `
		SELECT p.id, p.lapak_id, p.title, p.price_idr
		FROM market.products p
		JOIN market.lapak_profiles l ON l.id = p.lapak_id AND l.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	var p domain.Product
	err := db.QueryRow(ctx, query, productID).Scan(&p.ID, &p.LapakID, &p.Title, &p.PriceIDR)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		logger.Error("Failed to find product", logger.Err(err))
		return nil, err
	}
	return &p, nil
}

// InsertOrder creates the order shell. bid_id stays NULL: the schema's
// chk_orders_bid_source makes a bid_id legal only on a bid_* source.
func (r *Repository) InsertOrder(ctx context.Context, db DB, source, customerUserID, lapakID string) (string, error) {
	const query = `
		INSERT INTO market.orders (source, status, customer_user_id, lapak_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var id string
	err := db.QueryRow(ctx, query, source, domain.StatusPendingPayment, customerUserID, lapakID).Scan(&id)
	if err != nil {
		logger.Error("Failed to insert order", logger.Err(err))
		return "", err
	}
	return id, nil
}

// InsertOrderItem writes one item. name and unitPriceIDR are the caller's
// server-side snapshot of the catalog row; subtotal_idr is a GENERATED column
// and is never written. status defaults to 'unpaid' with payment_id NULL,
// which is the only pairing chk_order_items_paid_has_payment allows before a
// charge exists.
//
// productID and gigTierID are the two catalog sources, and at most one is
// non-nil (chk_order_items_one_source rejects both at once). One insert serves
// the product order, the gig order and the flow-B upsell rather than three
// that could drift about what a snapshot means.
func (r *Repository) InsertOrderItem(ctx context.Context, db DB, orderID string, productID, gigTierID *string, name string, unitPriceIDR int64, quantity int) error {
	const query = `
		INSERT INTO market.order_items (order_id, product_id, gig_tier_id, name, unit_price_idr, quantity)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if _, err := db.Exec(ctx, query, orderID, productID, gigTierID, name, unitPriceIDR, quantity); err != nil {
		logger.Error("Failed to insert order item", logger.Err(err))
		return err
	}
	return nil
}

// =====================================================================
// Read
// =====================================================================

// orderColumns and orderJoins are shared by the detail and list queries so the
// two cannot drift. chat_thread_id is a LEFT JOIN rather than a column: the
// schema keeps that relation on chat_threads.order_id and derives it here.
const orderColumns = `
	o.id, o.source, o.status,
	o.customer_user_id, COALESCE(cu.full_name, ''),
	l.id, l.name, l.rating,
	o.bid_id, ct.id,
	o.delivery_status, o.confirm_deadline_at, o.auto_confirmed, o.completed_at, o.created_at
`

const orderJoins = `
	FROM market.orders o
	JOIN core.users cu ON cu.id = o.customer_user_id
	JOIN market.lapak_profiles l ON l.id = o.lapak_id
	LEFT JOIN market.chat_threads ct ON ct.order_id = o.id
`

// ownership is the market domain's entire authorization model, expressed as a
// WHERE fragment: the caller is the order's customer, or its lapak. lapakID is
// a *string so a customer passes NULL — `o.lapak_id = NULL` evaluates to NULL,
// never true, so a customer can never match the lapak arm by accident.
const ownership = `(o.customer_user_id = $1 OR o.lapak_id = $2)`

func scanOrder(row pgx.Row) (*domain.Order, error) {
	var o domain.Order
	err := row.Scan(
		&o.ID, &o.Source, &o.Status,
		&o.Customer.ID, &o.Customer.FullName,
		&o.Lapak.ID, &o.Lapak.Name, &o.Lapak.Rating,
		&o.BidID, &o.ChatThreadID,
		&o.DeliveryStatus, &o.ConfirmDeadlineAt, &o.AutoConfirmed, &o.CompletedAt, &o.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// FindOrder loads one order with its items and payments, or ErrOrderNotFound
// when the caller is not a participant. The ownership test is in the query,
// not in a Go check after the fetch, so no code path ever holds the row and
// then decides not to show it.
func (r *Repository) FindOrder(ctx context.Context, db DB, orderID, userID string, lapakID *string) (*domain.Order, error) {
	query := `SELECT ` + orderColumns + orderJoins + ` WHERE ` + ownership + ` AND o.id = $3`

	order, err := scanOrder(db.QueryRow(ctx, query, userID, lapakID, orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		logger.Error("Failed to find order", logger.Err(err))
		return nil, err
	}

	if err := r.attach(ctx, db, []*domain.Order{order}); err != nil {
		return nil, err
	}
	return order, nil
}

// ListOrders returns one page of the caller's orders, newest first, with items
// and payments loaded. status is a *string: nil means no filter.
//
// ORDER BY carries `, o.id DESC` as a tiebreak, not decoration: created_at
// defaults to NOW(), which in Postgres is the TRANSACTION timestamp, so rows
// written in one transaction share a created_at byte-for-byte and their
// relative order would otherwise be undefined — a row could then appear on two
// LIMIT/OFFSET pages, or on none.
func (r *Repository) ListOrders(ctx context.Context, db DB, userID string, lapakID *string, status *string, page, limit int) ([]*domain.Order, error) {
	query := `SELECT ` + orderColumns + orderJoins + `
		WHERE ` + ownership + `
		  AND ($3::text IS NULL OR o.status = $3)
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT $4 OFFSET $5`

	rows, err := db.Query(ctx, query, userID, lapakID, status, limit, (page-1)*limit)
	if err != nil {
		logger.Error("Failed to list orders", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	orders := make([]*domain.Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			logger.Error("Failed to scan order", logger.Err(err))
			return nil, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate orders", logger.Err(err))
		return nil, err
	}

	if err := r.attach(ctx, db, orders); err != nil {
		return nil, err
	}
	return orders, nil
}

// CountByStatus is the whole of meta.counts: one COUNT(*) ... GROUP BY status
// over the caller's orders, deliberately WITHOUT the status predicate, so the
// counts are identical whether or not ?status= was passed and one request
// drives every tab badge.
//
// Every status is present, zeros included — an absent key is not the same as
// zero to a client rendering a badge per tab. "all" is the sum, which also
// gives the caller the page total without a second COUNT query.
func (r *Repository) CountByStatus(ctx context.Context, db DB, userID string, lapakID *string) (map[string]int64, error) {
	query := `SELECT o.status, COUNT(*) FROM market.orders o WHERE ` + ownership + ` GROUP BY o.status`

	rows, err := db.Query(ctx, query, userID, lapakID)
	if err != nil {
		logger.Error("Failed to count orders by status", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int64{"all": 0}
	for _, s := range domain.OrderStatuses {
		counts[s] = 0
	}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			logger.Error("Failed to scan order status count", logger.Err(err))
			return nil, err
		}
		counts[status] = n
		counts["all"] += n
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate order status counts", logger.Err(err))
		return nil, err
	}
	return counts, nil
}

// attach loads items and payments for a whole page of orders in two queries,
// not two per order.
func (r *Repository) attach(ctx context.Context, db DB, orders []*domain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]string, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}

	items, err := r.loadItems(ctx, db, ids)
	if err != nil {
		return err
	}
	payments, err := r.loadPayments(ctx, db, ids)
	if err != nil {
		return err
	}
	for _, o := range orders {
		o.Items = items[o.ID]
		o.Payments = payments[o.ID]
	}
	return nil
}

func (r *Repository) loadItems(ctx context.Context, db DB, orderIDs []string) (map[string][]domain.OrderItem, error) {
	const query = `
		SELECT oi.id, oi.order_id, oi.product_id, oi.gig_tier_id, gt.gig_id,
		       oi.name, oi.unit_price_idr, oi.quantity, oi.subtotal_idr,
		       oi.status, oi.created_at
		FROM market.order_items oi
		-- Amendment v1.0.3: gig_id travels with the item so a client can reach
		-- the tier's siblings for the flow-B upsell; nothing else in the
		-- contract resolves a tier back to its gig. LEFT, because product and
		-- bid items have no tier.
		LEFT JOIN market.gig_tiers gt ON gt.id = oi.gig_tier_id
		WHERE oi.order_id = ANY($1::uuid[])
		ORDER BY oi.created_at, oi.id
	`
	rows, err := db.Query(ctx, query, orderIDs)
	if err != nil {
		logger.Error("Failed to load order items", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]domain.OrderItem, len(orderIDs))
	for rows.Next() {
		var i domain.OrderItem
		if err := rows.Scan(&i.ID, &i.OrderID, &i.ProductID, &i.GigTierID, &i.GigID, &i.Name,
			&i.UnitPriceIDR, &i.Quantity, &i.SubtotalIDR, &i.Status, &i.CreatedAt); err != nil {
			logger.Error("Failed to scan order item", logger.Err(err))
			return nil, err
		}
		out[i.OrderID] = append(out[i.OrderID], i)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate order items", logger.Err(err))
		return nil, err
	}
	return out, nil
}

// loadPayments reads Payment.order_item_ids from the payment side of
// order_items.payment_id — the real foreign key — aggregated in SQL rather
// than with a third query.
func (r *Repository) loadPayments(ctx context.Context, db DB, orderIDs []string) (map[string][]domain.Payment, error) {
	const query = `
		SELECT p.id, p.order_id, p.amount_idr, p.paid_at,
		       COALESCE(ARRAY_AGG(oi.id::text) FILTER (WHERE oi.id IS NOT NULL), '{}') AS item_ids
		FROM market.payments p
		LEFT JOIN market.order_items oi ON oi.payment_id = p.id
		WHERE p.order_id = ANY($1::uuid[])
		GROUP BY p.id, p.order_id, p.amount_idr, p.paid_at
		ORDER BY p.paid_at, p.id
	`
	rows, err := db.Query(ctx, query, orderIDs)
	if err != nil {
		logger.Error("Failed to load payments", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]domain.Payment, len(orderIDs))
	for rows.Next() {
		var p domain.Payment
		if err := rows.Scan(&p.ID, &p.OrderID, &p.AmountIDR, &p.PaidAt, &p.OrderItemIDs); err != nil {
			logger.Error("Failed to scan payment", logger.Err(err))
			return nil, err
		}
		out[p.OrderID] = append(out[p.OrderID], p)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate payments", logger.Err(err))
		return nil, err
	}
	return out, nil
}

// =====================================================================
// Pay — the money path. Every method below is called inside ONE pgx
// transaction opened by the service; none of them is safe to call on the pool.
// =====================================================================

// FindCustomerOrder resolves {id} for the customer's verbs — /pay, /items and
// /confirm: the order must exist AND the caller must be its CUSTOMER. A lapak
// is a legitimate participant on its own orders for reads, but it is neither
// the payer nor the confirmer, so anyone who is not the customer gets
// ErrOrderNotFound and the 404 a stranger gets rather than a 403 that would
// confirm the order exists.
func (r *Repository) FindCustomerOrder(ctx context.Context, db DB, orderID, customerUserID string) error {
	const query = `SELECT 1 FROM market.orders WHERE id = $1 AND customer_user_id = $2`

	var one int
	err := db.QueryRow(ctx, query, orderID, customerUserID).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		logger.Error("Failed to load order for payment", logger.Err(err))
		return err
	}
	return nil
}

// LockWalletForUpdate takes a row lock on the customer's wallet and returns
// the balance as of the lock. This is what makes two concurrent pay calls on
// one order safe: both charge the same wallet, so the second blocks here and
// only reads the outstanding amount afterwards, by which time the first has
// committed and there is nothing left to charge.
//
// A user with no wallet row (self-signup, never provisioned) has no balance
// and no lock — but a zero balance cannot cover a positive charge, so that
// path always ends in 402 before anything is written.
func (r *Repository) LockWalletForUpdate(ctx context.Context, db DB, userID string) (int64, error) {
	const query = `SELECT balance_idr FROM market.wallets WHERE user_id = $1 FOR UPDATE`

	var balance int64
	err := db.QueryRow(ctx, query, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		logger.Error("Failed to lock wallet", logger.Err(err))
		return 0, err
	}
	return balance, nil
}

// SumUnpaid computes the charge server-side: the total of this order's unpaid
// items, and the exact ids that total covers. MUST be called after
// LockWalletForUpdate — under READ COMMITTED this statement takes a fresh
// snapshot, so once the lock is held it sees whatever a competing payment
// already committed.
//
// Returning the ids rather than re-selecting "unpaid" at update time means the
// items marked paid are precisely the items that were charged for.
func (r *Repository) SumUnpaid(ctx context.Context, db DB, orderID string) (int64, []string, error) {
	const query = `
		SELECT COALESCE(SUM(subtotal_idr), 0),
		       COALESCE(ARRAY_AGG(id::text ORDER BY created_at, id), '{}')
		FROM market.order_items
		WHERE order_id = $1 AND status = 'unpaid'
	`
	var total int64
	var ids []string
	if err := db.QueryRow(ctx, query, orderID).Scan(&total, &ids); err != nil {
		logger.Error("Failed to sum unpaid order items", logger.Err(err))
		return 0, nil, err
	}
	return total, ids, nil
}

// InsertPayment writes the charge row. amount must be > 0 —
// chk_payments_amount_positive rejects anything else, and the service has
// already refused a zero outstanding with 409.
func (r *Repository) InsertPayment(ctx context.Context, db DB, orderID string, amountIDR int64) (*domain.Payment, error) {
	const query = `
		INSERT INTO market.payments (order_id, amount_idr)
		VALUES ($1, $2)
		RETURNING id, order_id, amount_idr, paid_at
	`
	var p domain.Payment
	err := db.QueryRow(ctx, query, orderID, amountIDR).Scan(&p.ID, &p.OrderID, &p.AmountIDR, &p.PaidAt)
	if err != nil {
		logger.Error("Failed to insert payment", logger.Err(err))
		return nil, err
	}
	return &p, nil
}

// DebitWallet moves the balance and returns what it became, which is the value
// the ledger row must record as balance_after_idr. The subtraction happens in
// SQL on the locked row rather than by writing back a value Go computed, so
// the balance can never be built from a stale read.
func (r *Repository) DebitWallet(ctx context.Context, db DB, userID string, amountIDR int64) (int64, error) {
	const query = `
		UPDATE market.wallets
		SET balance_idr = balance_idr - $1, updated_at = NOW()
		WHERE user_id = $2
		RETURNING balance_idr
	`
	var balance int64
	if err := db.QueryRow(ctx, query, amountIDR, userID).Scan(&balance); err != nil {
		logger.Error("Failed to debit wallet", logger.Err(err))
		return 0, err
	}
	return balance, nil
}

// InsertLedgerEntry appends the money journal row. amountIDR is signed and
// must be negative for a charge; balanceAfterIDR must equal the wallet balance
// after the move. Append-only: nothing here ever updates or deletes a row.
func (r *Repository) InsertLedgerEntry(ctx context.Context, db DB, userID, entryType string, amountIDR, balanceAfterIDR int64, orderID, note string) error {
	const query = `
		INSERT INTO market.ledger_entries (user_id, type, amount_idr, balance_after_idr, order_id, note)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if _, err := db.Exec(ctx, query, userID, entryType, amountIDR, balanceAfterIDR, orderID, note); err != nil {
		logger.Error("Failed to insert ledger entry", logger.Err(err))
		return err
	}
	return nil
}

// MarkItemsPaid flips exactly the items this charge covered, pointing each at
// its payment row. Status and payment_id move together because
// chk_order_items_paid_has_payment says they are the same fact.
func (r *Repository) MarkItemsPaid(ctx context.Context, db DB, itemIDs []string, paymentID string) (int64, error) {
	const query = `
		UPDATE market.order_items
		SET status = 'paid', payment_id = $1, updated_at = NOW()
		WHERE id = ANY($2::uuid[]) AND status = 'unpaid'
	`
	tag, err := db.Exec(ctx, query, paymentID, itemIDs)
	if err != nil {
		logger.Error("Failed to mark order items paid", logger.Err(err))
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// MarkOrderPaid advances the order, but only from pending_payment.
//
// ponytail: a second payment on an order that is already `paid` (flow B's
// upsell) is a deliberate no-op here — the status is already right. An order
// further along than `paid` is left alone rather than dragged backwards; when
// BE-07 adds the upsell path it decides what an added item does to a
// later-stage order, and that decision belongs there, not in this UPDATE.
func (r *Repository) MarkOrderPaid(ctx context.Context, db DB, orderID string) error {
	const query = `
		UPDATE market.orders
		SET status = 'paid', updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'
	`
	if _, err := db.Exec(ctx, query, orderID); err != nil {
		logger.Error("Failed to mark order paid", logger.Err(err))
		return err
	}
	return nil
}

// EnsureChatThread opens the customer-lapak thread for a paid order
// (contract amendment v1.0.2). Idempotent by the schema's UNIQUE(order_id):
// phase 4's bid path may already have created the row, and a second payment on
// the same order must not attempt a duplicate.
//
// This opens the thread row only. Every chat endpoint is BE-06's.
func (r *Repository) EnsureChatThread(ctx context.Context, db DB, orderID string) error {
	const query = `
		INSERT INTO market.chat_threads (order_id)
		VALUES ($1)
		ON CONFLICT (order_id) DO NOTHING
	`
	if _, err := db.Exec(ctx, query, orderID); err != nil {
		logger.Error("Failed to ensure chat thread", logger.Err(err))
		return err
	}
	return nil
}

// =====================================================================
// Flow B — gig tiers, upsell, complete and the payout
// =====================================================================

// FindGigTier loads the tier an order item is priced from: the flow-B twin of
// FindProduct, and the same rule — the price and the name come from this row,
// never from the request.
//
// The joins are the guard FindProduct uses: a tier whose gig or lapak has been
// soft-deleted cannot back a new order, because that order would name a lapak
// no read can summarize.
func (r *Repository) FindGigTier(ctx context.Context, db DB, tierID string) (*domain.GigTier, error) {
	const query = `
		SELECT t.id, t.gig_id, g.lapak_id, t.name, t.price_idr
		FROM market.gig_tiers t
		JOIN market.gigs g ON g.id = t.gig_id AND g.deleted_at IS NULL
		JOIN market.lapak_profiles l ON l.id = g.lapak_id AND l.deleted_at IS NULL
		WHERE t.id = $1 AND t.deleted_at IS NULL
	`
	var t domain.GigTier
	err := db.QueryRow(ctx, query, tierID).Scan(&t.ID, &t.GigID, &t.LapakID, &t.Name, &t.PriceIDR)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGigTierNotFound
		}
		logger.Error("Failed to find gig tier", logger.Err(err))
		return nil, err
	}
	return &t, nil
}

// FindOrderForUpsell answers POST /orders/{id}/items' two questions in one
// round trip: may this caller add to this order (ownership, in the WHERE
// clause), and which gig is the order already about.
//
// gigID is NULL for a product or bid order, which is exactly why the service
// can reject "another tier of the same gig" for an order that has no gig: a
// NULL can never equal the new tier's gig id.
func (r *Repository) FindOrderForUpsell(ctx context.Context, db DB, orderID, customerUserID string) (string, *string, error) {
	const query = `
		SELECT o.status,
		       (SELECT gt.gig_id
		          FROM market.order_items oi
		          JOIN market.gig_tiers gt ON gt.id = oi.gig_tier_id
		         WHERE oi.order_id = o.id
		         LIMIT 1)
		FROM market.orders o
		WHERE o.id = $1 AND o.customer_user_id = $2
		FOR UPDATE OF o
	`
	var status string
	var gigID *string
	err := db.QueryRow(ctx, query, orderID, customerUserID).Scan(&status, &gigID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrOrderNotFound
		}
		logger.Error("Failed to load order for upsell", logger.Err(err))
		return "", nil, err
	}
	return status, gigID, nil
}

// FindLapakOrder resolves {id} for POST /orders/{id}/complete: it must exist
// AND the caller must be its LAPAK. The customer is a participant for reads,
// but completing is the worker's verb, so anyone else — the customer
// included — gets ErrOrderNotFound and a 404, the same answer a stranger gets.
func (r *Repository) FindLapakOrder(ctx context.Context, db DB, orderID, lapakID string) (string, error) {
	const query = `SELECT status FROM market.orders WHERE id = $1 AND lapak_id = $2 FOR UPDATE`

	var status string
	err := db.QueryRow(ctx, query, orderID, lapakID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrOrderNotFound
		}
		logger.Error("Failed to load order for completion", logger.Err(err))
		return "", err
	}
	return status, nil
}

// AutoConfirmSeconds reads the confirmation window from market.config.
//
// It is read, never hard-coded: QA edits that row to shorten the window, and a
// literal 60 here would silently ignore them. A missing key is an error rather
// than a zero — a zero-second window would auto-confirm every order on the
// sweeper's next tick.
//
// ponytail: one key, one query. The config submodule's typed reader wants all
// three keys and its own pool rather than this transaction, so borrowing it
// would cost a cross-module dependency to save four lines of SQL.
func (r *Repository) AutoConfirmSeconds(ctx context.Context, db DB) (int64, error) {
	const query = `SELECT value FROM market.config WHERE key = 'order_auto_confirm_seconds'`

	var seconds int64
	if err := db.QueryRow(ctx, query).Scan(&seconds); err != nil {
		logger.Error("Failed to read order_auto_confirm_seconds", logger.Err(err))
		return 0, err
	}
	return seconds, nil
}

// MarkAwaitingConfirmation is the whole of /complete's write: the status moves
// and the clock starts. No money moves here.
//
// The deadline is computed by the database from NOW(), not from a Go time, so
// it is on the same clock the sweeper compares against. AND status = 'paid'
// makes a second /complete a no-op rather than a restarted countdown: no rows
// come back and the service answers 409.
func (r *Repository) MarkAwaitingConfirmation(ctx context.Context, db DB, orderID string, seconds int64) (*time.Time, error) {
	const query = `
		UPDATE market.orders
		SET status = 'awaiting_confirmation',
		    confirm_deadline_at = NOW() + make_interval(secs => $2::double precision),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'paid'
		RETURNING confirm_deadline_at
	`
	var deadline time.Time
	err := db.QueryRow(ctx, query, orderID, seconds).Scan(&deadline)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotPaid
		}
		logger.Error("Failed to mark order awaiting confirmation", logger.Err(err))
		return nil, err
	}
	return &deadline, nil
}

// =====================================================================
// Payout — the second money path. Called only from the service's single
// completion transaction, shared by /confirm and the sweeper.
// =====================================================================

// LockOrderForCompletion takes a row lock on the order and returns its status
// as of the lock, plus the USER id behind its lapak profile.
//
// This lock is the double-payout guard. /confirm and the sweeper can target the
// same order at the same moment; both pass through here, so the loser blocks
// until the winner commits and then re-reads a row that already says
// completed. FOR UPDATE OF o keeps the lock on the order alone — the lapak
// profile is joined for its user_id, not locked.
//
// market.wallets is keyed by user_id and orders.lapak_id is a lapak_profiles
// id, so this join is what points the credit at the right wallet.
func (r *Repository) LockOrderForCompletion(ctx context.Context, db DB, orderID string) (string, string, error) {
	const query = `
		SELECT o.status, l.user_id
		FROM market.orders o
		JOIN market.lapak_profiles l ON l.id = o.lapak_id
		WHERE o.id = $1
		FOR UPDATE OF o
	`
	var status, lapakUserID string
	err := db.QueryRow(ctx, query, orderID).Scan(&status, &lapakUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrOrderNotFound
		}
		logger.Error("Failed to lock order for completion", logger.Err(err))
		return "", "", err
	}
	return status, lapakUserID, nil
}

// SumPaid totals the items the customer actually paid for — the amount the
// lapak is credited. /complete refuses an order with any outstanding item, so
// on this path it equals the order total; computing the payout from what was
// PAID keeps the credit honest even if that ever stops being true.
func (r *Repository) SumPaid(ctx context.Context, db DB, orderID string) (int64, error) {
	const query = `
		SELECT COALESCE(SUM(subtotal_idr), 0)
		FROM market.order_items
		WHERE order_id = $1 AND status = 'paid'
	`
	var total int64
	if err := db.QueryRow(ctx, query, orderID).Scan(&total); err != nil {
		logger.Error("Failed to sum paid order items", logger.Err(err))
		return 0, err
	}
	return total, nil
}

// CreditWallet moves money INTO a wallet and returns what the balance became,
// which is the value the payout ledger row must record.
//
// The arithmetic is in SQL on the row the statement itself locks, so two
// credits can never both build a balance from the same stale read. The upsert
// provisions a wallet for a lapak who has never had one — a payout is the
// first money most lapaks ever see, and dropping it because a row was missing
// would be a silent loss.
func (r *Repository) CreditWallet(ctx context.Context, db DB, userID string, amountIDR int64) (int64, error) {
	const query = `
		INSERT INTO market.wallets (user_id, balance_idr)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET balance_idr = market.wallets.balance_idr + EXCLUDED.balance_idr,
		    updated_at = NOW()
		RETURNING balance_idr
	`
	var balance int64
	if err := db.QueryRow(ctx, query, userID, amountIDR).Scan(&balance); err != nil {
		logger.Error("Failed to credit wallet", logger.Err(err))
		return 0, err
	}
	return balance, nil
}

// MarkOrderCompleted is the last statement of the payout and its last defence:
// AND status = 'awaiting_confirmation' means a row that somehow completed
// between the lock and here affects 0 rows, and the service rolls the credit
// back rather than paying twice.
//
// autoConfirmed is the ONLY thing that differs between the customer's /confirm
// and the sweeper. completed_at comes from NOW() because
// chk_orders_completed_at requires it, and chk_orders_auto_confirmed tolerates
// the flag only on a completed row.
func (r *Repository) MarkOrderCompleted(ctx context.Context, db DB, orderID string, autoConfirmed bool) (int64, error) {
	const query = `
		UPDATE market.orders
		SET status = 'completed', completed_at = NOW(), auto_confirmed = $2, updated_at = NOW()
		WHERE id = $1 AND status = 'awaiting_confirmation'
	`
	tag, err := db.Exec(ctx, query, orderID, autoConfirmed)
	if err != nil {
		logger.Error("Failed to mark order completed", logger.Err(err))
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// FindOverdueOrders is the sweeper's only query: orders whose confirmation
// window has elapsed. It hits idx_orders_confirm_deadline (partial, on
// status = 'awaiting_confirmation') and returns nothing almost every tick,
// which is the point — a sweep that finds nothing costs one index probe.
//
// NOW() is the database's clock, the same one MarkAwaitingConfirmation wrote
// the deadline from, so the window means the same thing at both ends however
// far the app server's clock has drifted.
//
// The LIMIT keeps one tick bounded; anything past it is swept on the next one.
func (r *Repository) FindOverdueOrders(ctx context.Context, db DB, limit int) ([]string, error) {
	const query = `
		SELECT id
		FROM market.orders
		WHERE status = 'awaiting_confirmation'
		  AND confirm_deadline_at IS NOT NULL
		  AND confirm_deadline_at <= NOW()
		ORDER BY confirm_deadline_at
		LIMIT $1
	`
	rows, err := db.Query(ctx, query, limit)
	if err != nil {
		logger.Error("Failed to find overdue orders", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			logger.Error("Failed to scan overdue order id", logger.Err(err))
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate overdue orders", logger.Err(err))
		return nil, err
	}
	return ids, nil
}
