package repository

import (
	"context"
	"errors"

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
func (r *Repository) InsertOrderItem(ctx context.Context, db DB, orderID string, productID *string, name string, unitPriceIDR int64, quantity int) error {
	const query = `
		INSERT INTO market.order_items (order_id, product_id, name, unit_price_idr, quantity)
		VALUES ($1, $2, $3, $4, $5)
	`
	if _, err := db.Exec(ctx, query, orderID, productID, name, unitPriceIDR, quantity); err != nil {
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

// FindPayableOrder resolves {id} for the pay path: it must exist AND the
// caller must be its CUSTOMER. A lapak is a legitimate participant on its own
// orders for reads, but it is not the payer, and POST /pay has no 403 in the
// contract — so anyone who is not the customer gets ErrOrderNotFound, the same
// answer a stranger gets.
func (r *Repository) FindPayableOrder(ctx context.Context, db DB, orderID, customerUserID string) error {
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
