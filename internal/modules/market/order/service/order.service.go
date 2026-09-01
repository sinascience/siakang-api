// Package service holds the order module's business logic. Unlike the wallet
// and me submodules, this one earns a service layer: creating an order is two
// inserts that must land together, and paying one is a seven-statement money
// transaction. Neither belongs in a handler.
package service

import (
	"context"
	"errors"

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

	// ErrGigNotSupported: gig_tier_id is flow B, phase 3, BE-07. The field is
	// accepted by the DTO so the contract's request shape round-trips, and
	// refused here rather than half-implemented.
	ErrGigNotSupported = errors.New("gig tier orders are not available yet")
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

// Create places a product order: status pending_payment, one unpaid item,
// priced from the database.
//
// No money moves here. The wallet is untouched until Pay.
func (s *Service) Create(ctx context.Context, userID, productID, gigTierID string, quantity int) (*domain.Order, error) {
	if gigTierID != "" {
		return nil, ErrGigNotSupported
	}

	// Only customers create orders. The persona is the caller's own lapak
	// profile, resolved from the JWT user id — never from the request.
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	if lapakID != "" {
		return nil, ErrLapakCannotOrder
	}

	// The price and the name come from this row, not from the request body.
	// A client-sent price is not validated or compared — it is ignored, and
	// the DTO has no field to carry one.
	product, err := s.repo.FindProduct(ctx, s.repo.Pool(), productID)
	if err != nil {
		return nil, err
	}

	if quantity < 1 {
		quantity = 1
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

	orderID, err := s.repo.InsertOrder(ctx, tx, domain.SourceProduct, userID, product.LapakID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.InsertOrderItem(ctx, tx, orderID, &product.ID, product.Title, product.PriceIDR, quantity); err != nil {
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
		logger.String("customer_user_id", userID),
		logger.String("lapak_id", product.LapakID),
		logger.String("product_id", product.ID),
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
	if err := s.repo.FindPayableOrder(ctx, tx, orderID, userID); err != nil {
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
