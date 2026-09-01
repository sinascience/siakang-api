// Package service holds the order module's business logic. Unlike the wallet
// and me submodules, this one earns a service layer: creating an order is two
// inserts that must land together, and paying one is a seven-statement money
// transaction. Neither belongs in a handler.
package service

import (
	"context"
	"errors"
	"time"

	"siakang-api/internal/modules/market/order/domain"
	"siakang-api/internal/modules/market/order/repository"
	"siakang-api/pkg/logger"
)

var (
	// ErrLapakCannotOrder is the contract's 403 on POST /orders: only
	// customers create orders.
	ErrLapakCannotOrder = errors.New("lapak accounts cannot create orders")

	// ErrNothingOutstanding is the contract's 409 on POST /orders/{id}/pay.
	ErrNothingOutstanding = errors.New("nothing outstanding to pay on this order")

	// ErrInsufficientBalance is the contract's 402. It is raised BEFORE any
	// write, never caught from the wallet's non-negative CHECK constraint —
	// that constraint is the backstop, not the control flow.
	ErrInsufficientBalance = errors.New("insufficient wallet balance")

	// ErrOrderNotAcceptingItems is the contract's 409 on POST /orders/{id}/items
	// for an order already awaiting_confirmation, completed or cancelled: the
	// work is done or abandoned, so there is nothing left to upsell into.
	ErrOrderNotAcceptingItems = errors.New("order is not in a state that accepts more items")

	// ErrTierDifferentGig is the same endpoint's other 409. The upsell is
	// defined as another tier of the SAME gig — a tier from elsewhere in the
	// catalog would put two lapaks' work on one order and one lapak's wallet.
	ErrTierDifferentGig = errors.New("gig tier belongs to a different gig")

	// ErrOutstandingItems is the contract's 409 on POST /orders/{id}/complete:
	// an added-but-unpaid upsell item blocks completion, because completing is
	// what starts the clock on paying the lapak.
	ErrOutstandingItems = errors.New("order has unpaid items")

	// ErrNotAwaitingConfirmation is the contract's 409 on
	// POST /orders/{id}/confirm. An order already `completed` is NOT this
	// error: the sweeper getting there first is a race the customer wins a
	// 200 for, not a failure.
	ErrNotAwaitingConfirmation = errors.New("order is not awaiting confirmation")
)

// The sweeper's two knobs.
const (
	// SweepInterval is how often the auto-confirm sweeper looks for overdue
	// orders. The contract asks for <= 10s; the seeded window is 60s, so 5s
	// makes the transition directly observable while costing one index probe
	// per tick on a partial index that is empty almost always.
	SweepInterval = 5 * time.Second

	// sweepBatch bounds one tick. Whatever is left waits ~5 seconds.
	sweepBatch = 100
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// PayResult is what POST /orders/{id}/pay produces: the refreshed order, the
// charge that was just written, and the customer's balance after it.
type PayResult struct {
	Order            *domain.Order
	Payment          *domain.Payment
	WalletBalanceIDR int64
}

// Create places an order for one catalog line: status pending_payment, one
// unpaid item, priced from the database.
//
// Two flows, one transaction. Flow A prices from a product; flow B (criterion
// 2) prices from a gig tier and takes its lapak from the tier's gig. Only the
// lookup differs, so only the lookup branches — the order and its item are
// written once, below.
//
// No money moves here. The wallet is untouched until Pay.
func (s *Service) Create(ctx context.Context, userID, productID, gigTierID string, quantity int) (*domain.Order, error) {
	// Only customers create orders. The persona is the caller's own lapak
	// profile, resolved from the JWT user id — never from the request.
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	if lapakID != "" {
		return nil, ErrLapakCannotOrder
	}

	// The price and the name come from the catalog row, not from the request
	// body. A client-sent price is not validated or compared — it is ignored,
	// and the DTO has no field to carry one.
	var (
		source           string
		orderLapakID     string
		itemProductID    *string
		itemGigTierID    *string
		itemName         string
		itemUnitPriceIDR int64
	)

	if gigTierID != "" {
		tier, err := s.repo.FindGigTier(ctx, s.repo.Pool(), gigTierID)
		if err != nil {
			return nil, err
		}
		source, orderLapakID = domain.SourceGig, tier.LapakID
		itemGigTierID, itemName, itemUnitPriceIDR = &tier.ID, tier.Name, tier.PriceIDR
		// "quantity: products only; ignored for gig tiers" (contract). A tier
		// is a piece of work, not a unit — buying the consultation twice is
		// two visits, i.e. two orders.
		quantity = 1
	} else {
		product, err := s.repo.FindProduct(ctx, s.repo.Pool(), productID)
		if err != nil {
			return nil, err
		}
		source, orderLapakID = domain.SourceProduct, product.LapakID
		itemProductID, itemName, itemUnitPriceIDR = &product.ID, product.Title, product.PriceIDR
		if quantity < 1 {
			quantity = 1
		}
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin order transaction", logger.Err(err))
		return nil, err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to defer
	// unconditionally and it is what guarantees an order never exists without
	// its item.
	defer func() { _ = tx.Rollback(ctx) }()

	orderID, err := s.repo.InsertOrder(ctx, tx, source, userID, orderLapakID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.InsertOrderItem(ctx, tx, orderID, itemProductID, itemGigTierID,
		itemName, itemUnitPriceIDR, quantity); err != nil {
		return nil, err
	}

	// Read the order back inside the same transaction, so the response is
	// exactly what was committed.
	order, err := s.repo.FindOrder(ctx, tx, orderID, userID, nil)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit order transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Order created",
		logger.String("order_id", order.ID),
		logger.String("source", source),
		logger.String("customer_user_id", userID),
		logger.String("lapak_id", orderLapakID),
		logger.Int64("total_idr", order.TotalIDR()))

	return order, nil
}

// Pay charges the customer's wallet for every unpaid item on the order, in ONE
// transaction. Either all six writes land or none of them do.
//
// Order of operations, and why:
//
//  1. resolve the order and the payer together — a non-customer gets
//     ErrOrderNotFound, so the endpoint never confirms someone else's order;
//  2. lock the wallet row (SELECT ... FOR UPDATE);
//  3. only THEN compute the outstanding amount, so a concurrent payment that
//     committed while we waited on the lock is visible and we charge 0;
//  4. compare balance to amount and return 402 before writing anything;
//  5. payment, ledger, wallet, items, order, chat thread;
//  6. commit.
//
// Steps 2 and 3 are the concurrency guard: two pay calls on one order always
// contend for the same wallet row, because only the order's customer can pay
// it. The loser blocks at step 2, sees no unpaid items at step 3, and gets 409
// instead of a second charge.
func (s *Service) Pay(ctx context.Context, userID, orderID string) (*PayResult, error) {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin payment transaction", logger.Err(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Existence and ownership in one query.
	if err := s.repo.FindCustomerOrder(ctx, tx, orderID, userID); err != nil {
		return nil, err
	}

	// 2. Serialize against any other payment from this wallet.
	balance, err := s.repo.LockWalletForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	// 3. The amount is computed from the item rows, after the lock. Nothing
	// the client sent is involved — POST /pay has no body at all.
	amount, itemIDs, err := s.repo.SumUnpaid(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, ErrNothingOutstanding
	}

	// 4. 402 before any write. The rollback below leaves zero rows behind:
	// the transaction has issued nothing but SELECTs at this point.
	if balance < amount {
		logger.Warn("Payment refused: insufficient balance",
			logger.String("order_id", orderID),
			logger.String("user_id", userID),
			logger.Int64("balance_idr", balance),
			logger.Int64("amount_idr", amount))
		return nil, ErrInsufficientBalance
	}

	// 5. The writes.
	payment, err := s.repo.InsertPayment(ctx, tx, orderID, amount)
	if err != nil {
		return nil, err
	}

	newBalance, err := s.repo.DebitWallet(ctx, tx, userID, amount)
	if err != nil {
		return nil, err
	}

	// Signed negative: money left this wallet. balance_after_idr is what the
	// UPDATE actually produced, not a number recomputed here.
	if err := s.repo.InsertLedgerEntry(ctx, tx, userID, domain.LedgerTypeOrderPayment,
		-amount, newBalance, orderID, "Order payment"); err != nil {
		return nil, err
	}

	marked, err := s.repo.MarkItemsPaid(ctx, tx, itemIDs, payment.ID)
	if err != nil {
		return nil, err
	}
	// A mismatch means something changed the items between step 3 and here
	// despite the lock. Rolling back is the only honest response: the charge
	// would otherwise cover a different set of items than it was computed
	// from. Belt-and-braces — the wallet lock should make this unreachable.
	if marked != int64(len(itemIDs)) {
		logger.Error("Payment aborted: unpaid items changed mid-transaction",
			logger.String("order_id", orderID),
			logger.Int("expected", len(itemIDs)),
			logger.Int64("marked", marked))
		return nil, errors.New("order items changed during payment")
	}
	// The INSERT ... RETURNING could not know these yet — the items only point
	// at the payment one statement later. They are the exact ids the charge was
	// computed from, so PayResult.payment.order_item_ids is filled from the
	// same list rather than re-read.
	payment.OrderItemIDs = itemIDs

	if err := s.repo.MarkOrderPaid(ctx, tx, orderID); err != nil {
		return nil, err
	}

	// Amendment v1.0.2: any order reaching paid opens its chat thread,
	// server-side, in this transaction.
	if err := s.repo.EnsureChatThread(ctx, tx, orderID); err != nil {
		return nil, err
	}

	order, err := s.repo.FindOrder(ctx, tx, orderID, userID, nil)
	if err != nil {
		return nil, err
	}

	// 6.
	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit payment transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Order paid",
		logger.String("order_id", orderID),
		logger.String("payment_id", payment.ID),
		logger.String("user_id", userID),
		logger.Int64("amount_idr", amount),
		logger.Int64("balance_after_idr", newBalance),
		logger.Int("items_paid", len(itemIDs)))

	return &PayResult{Order: order, Payment: payment, WalletBalanceIDR: newBalance}, nil
}

// AddItem is the flow-B upsell (criterion 3): another tier of the SAME gig,
// added to the SAME order as a new unpaid item. No new order id, and no money —
// POST /pay charges it next, producing the second payment row against this
// order.
//
// There is no proposal to accept: the lapak upsells in the order's chat thread
// and the customer calling this endpoint IS the agreement, so there is no
// proposal state for the two sides to disagree about.
//
// The order's row lock is taken first (FindOrderForUpsell ... FOR UPDATE). It
// is what stops a lapak's /complete from passing its "nothing outstanding"
// test in the instant between this call reading the status and inserting the
// item, which would leave a countdown running on a half-paid order.
func (s *Service) AddItem(ctx context.Context, userID, orderID, gigTierID string) (*domain.Order, error) {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin add-item transaction", logger.Err(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Existence and ownership together: a non-customer gets ErrOrderNotFound
	// and a 404, never a 403 that would confirm the order exists.
	status, gigID, err := s.repo.FindOrderForUpsell(ctx, tx, orderID, userID)
	if err != nil {
		return nil, err
	}
	// "Allowed while the order is paid and not yet completed." pending_payment
	// is allowed too: the contract's 409 names awaiting_confirmation,
	// completed and cancelled, and adding a tier before paying is the same
	// order the customer would get by paying twice — one charge instead of
	// two. The three states it does name are the ones where the work is over.
	if status != domain.StatusPaid && status != domain.StatusPendingPayment {
		return nil, ErrOrderNotAcceptingItems
	}

	tier, err := s.repo.FindGigTier(ctx, tx, gigTierID)
	if err != nil {
		return nil, err
	}
	// gigID is NULL on a product or bid order, so that order can never match a
	// tier and this one check covers both "different gig" and "not a gig
	// order at all".
	if gigID == nil || *gigID != tier.GigID {
		return nil, ErrTierDifferentGig
	}

	if err := s.repo.InsertOrderItem(ctx, tx, orderID, nil, &tier.ID, tier.Name, tier.PriceIDR, 1); err != nil {
		return nil, err
	}

	order, err := s.repo.FindOrder(ctx, tx, orderID, userID, nil)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit add-item transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Order item added",
		logger.String("order_id", orderID),
		logger.String("gig_tier_id", tier.ID),
		logger.Int64("unit_price_idr", tier.PriceIDR),
		logger.Int64("outstanding_idr", order.OutstandingIDR()))

	return order, nil
}

// Complete is the lapak saying the work is done (criterion 4): the order moves
// to awaiting_confirmation and the confirmation window starts.
//
// NO MONEY MOVES HERE. The payout happens when the order completes, by either
// of the two paths below.
//
// There is deliberately NO order-source restriction: the contract gates on
// outstanding_idr == 0, not on gig-vs-product, so a lapak may complete a
// product order (product ruling 2026-09-02, plan.md rev 7 criterion 4). Adding
// the source guard an earlier plan revision implied would be an off-contract
// behaviour deviation.
func (s *Service) Complete(ctx context.Context, userID, orderID string) (*domain.Order, error) {
	// Completing is the worker's verb. A caller with no lapak profile — every
	// customer, including this order's own — owns no order as a lapak, so the
	// answer is the 404 a stranger gets rather than a 403 that would confirm
	// the order exists.
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	if lapakID == "" {
		return nil, repository.ErrOrderNotFound
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin complete transaction", logger.Err(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Existence, ownership and the row lock in one statement.
	status, err := s.repo.FindLapakOrder(ctx, tx, orderID, lapakID)
	if err != nil {
		return nil, err
	}
	if status != domain.StatusPaid {
		return nil, repository.ErrOrderNotPaid
	}

	// The 409 criterion 4 asks for: an added-but-unpaid upsell item blocks
	// completion. Reusing the pay path's own sum means "outstanding" cannot
	// mean two different things at the two ends of the flow.
	outstanding, _, err := s.repo.SumUnpaid(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if outstanding > 0 {
		logger.Warn("Completion refused: order has unpaid items",
			logger.String("order_id", orderID),
			logger.Int64("outstanding_idr", outstanding))
		return nil, ErrOutstandingItems
	}

	// Read from market.config, never hard-coded: QA shortens this window by
	// editing the row.
	seconds, err := s.repo.AutoConfirmSeconds(ctx, tx)
	if err != nil {
		return nil, err
	}
	deadline, err := s.repo.MarkAwaitingConfirmation(ctx, tx, orderID, seconds)
	if err != nil {
		return nil, err
	}

	order, err := s.repo.FindOrder(ctx, tx, orderID, userID, &lapakID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit complete transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Order awaiting confirmation",
		logger.String("order_id", orderID),
		logger.String("lapak_id", lapakID),
		logger.Int64("auto_confirm_seconds", seconds),
		logger.String("confirm_deadline_at", deadline.UTC().Format(time.RFC3339)))

	return order, nil
}

// Confirm is the customer's half of criterion 5: the order completes and the
// lapak is paid.
//
// It owns no money logic of its own. The payout is completeOrder, which the
// sweeper calls with exactly one argument different, so the two paths cannot
// drift about what the lapak is owed.
//
// Idempotent against the sweeper: if the window elapsed first the order is
// already completed, completeOrder credits nothing, and the customer gets 200
// with the already-completed order — auto_confirmed still true, because that
// is what actually happened.
func (s *Service) Confirm(ctx context.Context, userID, orderID string) (*domain.Order, error) {
	// Ownership before the payout, on the pool: customer_user_id never changes
	// for the life of an order, so there is nothing here for the transaction
	// below to protect.
	if err := s.repo.FindCustomerOrder(ctx, s.repo.Pool(), orderID, userID); err != nil {
		return nil, err
	}

	if err := s.completeOrder(ctx, orderID, false); err != nil {
		return nil, err
	}

	return s.repo.FindOrder(ctx, s.repo.Pool(), orderID, userID, nil)
}

// completeOrder is THE payout, and the only place a lapak's wallet is ever
// credited. It has exactly two callers — the customer's Confirm and the
// sweeper — and autoConfirmed is the only thing that differs between them. A
// second copy of this transaction would eventually drift, and the drift would
// be about money.
//
// One transaction, in this order, and the order is the point:
//
//  1. lock the ORDER row and re-read its status inside the transaction;
//  2. already completed → return, crediting nothing. This is the whole
//     defence against a double payout: /confirm and the sweeper can fire on
//     one order at the same instant, and the loser arrives here to find the
//     work done;
//  3. not awaiting confirmation → 409, nothing written;
//  4. the amount is the PAID total, computed from the item rows;
//  5. credit the lapak's wallet, which is keyed by user_id — resolved through
//     lapak_profiles at step 1, because orders.lapak_id is a profile id and
//     crediting it directly would pay nobody;
//  6. append the positive `payout` ledger row with the balance the UPDATE
//     actually produced;
//  7. flip the order, guarded on awaiting_confirmation once more; anything
//     other than one row affected rolls the credit back.
//
// The caller does not get the order back: Confirm re-reads it under its own
// ownership scope and the sweeper has nobody to answer. That keeps this
// function free of any notion of who is asking.
func (s *Service) completeOrder(ctx context.Context, orderID string, autoConfirmed bool) error {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin payout transaction", logger.Err(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1.
	status, lapakUserID, err := s.repo.LockOrderForCompletion(ctx, tx, orderID)
	if err != nil {
		return err
	}

	// 2. The other caller got here first. Do not pay twice.
	if status == domain.StatusCompleted {
		logger.Info("Order already completed; payout skipped",
			logger.String("order_id", orderID),
			logger.Bool("auto_confirmed_attempt", autoConfirmed))
		return nil
	}

	// 3.
	if status != domain.StatusAwaitingConfirmation {
		return ErrNotAwaitingConfirmation
	}

	// 4.
	amount, err := s.repo.SumPaid(ctx, tx, orderID)
	if err != nil {
		return err
	}

	var balanceAfter int64
	if amount > 0 {
		// 5.
		balanceAfter, err = s.repo.CreditWallet(ctx, tx, lapakUserID, amount)
		if err != nil {
			return err
		}
		// 6. Signed positive: money arrived. balance_after_idr is what the
		// UPDATE produced, not a number recomputed here.
		if err := s.repo.InsertLedgerEntry(ctx, tx, lapakUserID, domain.LedgerTypePayout,
			amount, balanceAfter, orderID, "Order payout"); err != nil {
			return err
		}
	} else {
		// Unreachable through the API — /complete refuses an order that is not
		// `paid`. Completing anyway beats a sweeper that retries the same
		// unpayable order every five seconds forever, and a zero-amount ledger
		// row would be rejected by chk_ledger_entries_amount_non_zero anyway.
		logger.Warn("Completing an order with nothing paid; no payout written",
			logger.String("order_id", orderID))
	}

	// 7.
	completed, err := s.repo.MarkOrderCompleted(ctx, tx, orderID, autoConfirmed)
	if err != nil {
		return err
	}
	if completed != 1 {
		logger.Error("Payout aborted: order left awaiting_confirmation mid-transaction",
			logger.String("order_id", orderID),
			logger.Int64("rows_affected", completed))
		return errors.New("order status changed during completion")
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit payout transaction", logger.Err(err))
		return err
	}

	// The audit trail QA reads when it asks what happened while nobody was
	// looking.
	logger.Info("Order completed and lapak paid",
		logger.String("order_id", orderID),
		logger.String("lapak_user_id", lapakUserID),
		logger.Int64("amount_idr", amount),
		logger.Int64("balance_after_idr", balanceAfter),
		logger.Bool("auto_confirmed", autoConfirmed))

	return nil
}

// Sweep completes every order whose confirmation window has elapsed, and is
// criterion 6: the customer never clicked, and the lapak is paid anyway.
//
// One transaction PER ORDER, none held across the batch, so a row that cannot
// be completed is logged and skipped instead of blocking the rest — and the
// next tick tries it again.
//
// It returns the number completed so a caller (or a test) can see whether a
// tick did anything; the ticker ignores it.
func (s *Service) Sweep(ctx context.Context) int {
	ids, err := s.repo.FindOverdueOrders(ctx, s.repo.Pool(), sweepBatch)
	if err != nil {
		// Already logged in the repository. A failed sweep is not fatal: the
		// next tick is five seconds away.
		return 0
	}

	swept := 0
	for _, id := range ids {
		// true — and ONLY here. auto_confirmed is a claim that the customer
		// did not click, and FE renders it as a distinct label, so no other
		// call site may pass it.
		if err := s.completeOrder(ctx, id, true); err != nil {
			logger.Error("Sweeper failed to auto-confirm order",
				logger.String("order_id", id), logger.Err(err))
			continue
		}
		swept++
	}
	if swept > 0 {
		logger.Info("Sweeper auto-confirmed overdue orders", logger.Int("count", swept))
	}
	return swept
}

// RunSweeper ticks Sweep for the process lifetime. It blocks, so the module's
// Initialize starts it in one goroutine — one, for the whole server, because
// two would race each other over the same rows for no gain.
//
// Nothing is held between ticks: each tick opens its own transactions and
// closes them. A tick that finds nothing costs one probe of a partial index
// and logs nothing, which is why it can afford to run every few seconds
// forever.
func (s *Service) RunSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("Order auto-confirm sweeper started",
		logger.String("interval", interval.String()))

	for {
		select {
		case <-ctx.Done():
			logger.Info("Order auto-confirm sweeper stopped")
			return
		case <-ticker.C:
			s.Sweep(ctx)
		}
	}
}

// Get returns one order, or repository.ErrOrderNotFound when the caller is
// neither its customer nor its lapak.
func (s *Service) Get(ctx context.Context, userID, orderID string) (*domain.Order, error) {
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	return s.repo.FindOrder(ctx, s.repo.Pool(), orderID, userID, nullable(lapakID))
}

// List returns the caller's page of orders plus meta.counts.
//
// The persona scoping is the server's: a customer sees orders they placed, a
// lapak sees orders placed against them, and the query parameter that would
// let a caller choose does not exist.
//
// counts ignores the status filter on purpose, so the same request drives
// every tab badge. total is read straight out of counts — the filtered bucket,
// or "all" when unfiltered — which is why there is no separate COUNT query.
func (s *Service) List(ctx context.Context, userID, status string, page, limit int) ([]*domain.Order, map[string]int64, int64, error) {
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, nil, 0, err
	}
	lapak := nullable(lapakID)

	counts, err := s.repo.CountByStatus(ctx, s.repo.Pool(), userID, lapak)
	if err != nil {
		return nil, nil, 0, err
	}

	total := counts["all"]
	if status != "" {
		total = counts[status]
	}

	orders, err := s.repo.ListOrders(ctx, s.repo.Pool(), userID, lapak, nullable(status), page, limit)
	if err != nil {
		return nil, nil, 0, err
	}
	return orders, counts, total, nil
}

// nullable turns "no value" into a SQL NULL, which is what makes the ownership
// and status predicates behave: a customer's NULL lapak id can never equal a
// real one, and a NULL status disables the filter entirely.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
