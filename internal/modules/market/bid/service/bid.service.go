// Package service holds the bid module's business logic. This module earns a
// service layer the way order does and product does not: fee-then-match, the
// no_match refund and the award transaction are real logic, and none of it
// belongs in a handler.
package service

import (
	"context"
	"errors"

	"siakang-api/internal/modules/market/bid/domain"
	"siakang-api/internal/modules/market/bid/repository"
	"siakang-api/pkg/logger"
)

var (
	// ErrLapakCannotBid is POST /bids' 403: only customers create bids.
	ErrLapakCannotBid = errors.New("lapak accounts cannot create bids")

	// ErrNotLapak is POST /bids/{id}/offers' 403: only lapaks place offers.
	ErrNotLapak = errors.New("only lapak accounts can place offers")

	// ErrNotCustomer is the 403 on /confirm and /award — the caller can see
	// this bid but it is not theirs to advance.
	ErrNotCustomer = errors.New("caller is not this bid's customer")

	// ErrNotMatchedLapak is /accept's 403. It is deliberately NOT the same
	// question as ErrNotCustomer: the contract's text is "caller is not the
	// matched lapak", so any lapak has standing to be told that, while a
	// caller with no relationship to the bid still gets 404.
	ErrNotMatchedLapak = errors.New("caller is not the matched lapak")

	// ErrInsufficientBalance is the contract's 402 on POST /bids (the
	// automatic fee) and on /award (the manual fee). It is raised BEFORE any
	// write — never caught from the wallet's non-negative CHECK, which is the
	// backstop, not the control flow.
	ErrInsufficientBalance = errors.New("insufficient wallet balance")

	// ErrWrongStatus is the 409 shared by /confirm, /accept, /offers and
	// /award: the bid is real and the caller is entitled, but the step does
	// not apply to the status the bid is actually in.
	ErrWrongStatus = errors.New("bid is not in the expected status for this step")

	// ErrBidChanged means the bid moved between the row lock and the guarded
	// UPDATE. Unreachable while the lock is held; rolling back is the only
	// honest answer if it ever is reached, because the alternative is a second
	// order or a second fee.
	ErrBidChanged = errors.New("bid status changed during the transaction")
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// ListCategories backs GET /market/v1/bid-categories.
func (s *Service) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx, s.repo.Pool())
}

// Create posts a bid in either mode — one endpoint, because both modes produce
// the same resource and differ only in how the counterparty is chosen and when
// the platform fee lands.
//
// **Manual** is free to post: status `open`, fee_paid_idr 0. The
// bid_manual_fee_idr is charged at award.
//
// **Automatic** charges bid_auto_fee_idr BEFORE it searches. That ordering is
// criterion BE-08.1 itself, not an implementation detail, and it is why the
// statements below run in exactly this sequence:
//
//  1. lock the wallet and compare it to the fee — a refusal here writes
//     nothing and, crucially, runs NO search;
//  2. insert the bid, because the ledger row's bid_id needs a bid to point at;
//  3. debit the wallet and append the `platform_fee` row;
//  4. only NOW search for the nearest available lapak in the category;
//  5. matched → `proposed` with the lapak and distance recorded;
//     nothing → the bid stays `no_match` and the fee is refunded IN THIS SAME
//     TRANSACTION, because charging for a match that never happened is a real
//     money bug, not a corner case.
//
// The bid is inserted at `no_match` and moved to `proposed` on success rather
// than the other way round: an automatic bid that has matched nobody yet has
// matched nobody, and the schema has no `matching` status to borrow — matching
// is synchronous inside this request, so no client ever observes the interim
// row anyway.
func (s *Service) Create(ctx context.Context, userID, mode, categoryID, title, description string, budgetIDR int64, lat, lng *float64) (*domain.Bid, error) {
	// Only customers create bids. The persona comes from the caller's own
	// lapak profile, resolved from the JWT user id — never from the request.
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	if lapakID != "" {
		return nil, ErrLapakCannotBid
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin bid transaction", logger.Err(err))
		return nil, err
	}
	// Rollback after a successful Commit is a no-op, so deferring it
	// unconditionally is what guarantees the 402 path leaves nothing behind.
	defer func() { _ = tx.Rollback(ctx) }()

	category, err := s.repo.FindCategory(ctx, tx, categoryID)
	if err != nil {
		return nil, err
	}

	var bidID string
	if mode == domain.ModeManual {
		bidID, err = s.repo.InsertBid(ctx, tx, domain.ModeManual, domain.StatusOpen,
			category.ID, userID, title, description, budgetIDR, lat, lng, 0)
		if err != nil {
			return nil, err
		}
		logger.Info("Manual bid posted",
			logger.String("bid_id", bidID),
			logger.String("customer_user_id", userID),
			logger.String("category", category.Slug),
			logger.Int64("budget_idr", budgetIDR))
	} else {
		bidID, err = s.createAuto(ctx, tx, userID, category, title, description, budgetIDR, lat, lng)
		if err != nil {
			return nil, err
		}
	}

	// Read the bid back inside the same transaction, so the response is
	// exactly what was committed.
	bid, err := s.repo.FindBid(ctx, tx, bidID, userID, nil)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit bid transaction", logger.Err(err))
		return nil, err
	}
	return bid, nil
}

// createAuto is steps 1-5 above, inside the caller's transaction.
func (s *Service) createAuto(ctx context.Context, tx repository.DB, userID string, category *domain.Category, title, description string, budgetIDR int64, lat, lng *float64) (string, error) {
	// The fee is a market.config row, read every time. QA edits it to prove
	// the platform is not hard-coding 2500.
	fee, err := s.repo.FeeIDR(ctx, tx, domain.ConfigKeyAutoFee)
	if err != nil {
		return "", err
	}

	// 1. Lock, then compare. Nothing has been written and no search has run,
	// so the rollback on this path leaves no bid row and no ledger row.
	balance, err := s.repo.LockWalletForUpdate(ctx, tx, userID)
	if err != nil {
		return "", err
	}
	if balance < fee {
		logger.Warn("Automatic bid refused: insufficient balance for the fee",
			logger.String("user_id", userID),
			logger.Int64("balance_idr", balance),
			logger.Int64("fee_idr", fee))
		return "", ErrInsufficientBalance
	}

	// 2.
	bidID, err := s.repo.InsertBid(ctx, tx, domain.ModeAuto, domain.StatusNoMatch,
		category.ID, userID, title, description, budgetIDR, lat, lng, fee)
	if err != nil {
		return "", err
	}

	// 3. THE FEE IS CHARGED HERE — before the search below, which is the
	// criterion.
	afterFee, err := s.repo.MoveWallet(ctx, tx, userID, -fee)
	if err != nil {
		return "", err
	}
	if err := s.repo.InsertLedgerEntry(ctx, tx, userID, domain.LedgerTypePlatformFee,
		-fee, afterFee, bidID, "Automatic bid matching fee"); err != nil {
		return "", err
	}
	logger.Info("Automatic bid fee charged before matching",
		logger.String("bid_id", bidID),
		logger.String("user_id", userID),
		logger.Int64("fee_idr", fee),
		logger.Int64("balance_after_idr", afterFee))

	// 4.
	match, err := s.repo.FindNearestLapak(ctx, tx, category.ID, *lat, *lng)
	if err != nil {
		return "", err
	}

	// 5a. No candidate: refund inside this transaction and leave the bid at
	// no_match with fee_paid_idr back to 0. The two ledger rows share a
	// byte-identical created_at — NOW() is the transaction timestamp — which
	// is why the ledger read orders by `created_at DESC, id DESC`.
	if match == nil {
		afterRefund, err := s.repo.MoveWallet(ctx, tx, userID, fee)
		if err != nil {
			return "", err
		}
		if err := s.repo.InsertLedgerEntry(ctx, tx, userID, domain.LedgerTypeRefund,
			fee, afterRefund, bidID, "Automatic bid fee refund (no match)"); err != nil {
			return "", err
		}
		if err := s.repo.SetBidFeePaid(ctx, tx, bidID, 0); err != nil {
			return "", err
		}
		logger.Info("Automatic bid found no candidate; fee refunded in the same transaction",
			logger.String("bid_id", bidID),
			logger.String("category", category.Slug),
			logger.Int64("refund_idr", fee),
			logger.Int64("balance_after_idr", afterRefund))
		return bidID, nil
	}

	// 5b.
	if err := s.repo.MarkBidProposed(ctx, tx, bidID, match.Lapak.ID, match.DistanceKM); err != nil {
		return "", err
	}
	logger.Info("Automatic bid matched",
		logger.String("bid_id", bidID),
		logger.String("matched_lapak_id", match.Lapak.ID),
		logger.String("matched_lapak", match.Lapak.Name),
		logger.Float64("matched_distance_km", match.DistanceKM),
		logger.Float64("matched_lapak_rating", match.Lapak.Rating))
	return bidID, nil
}

// Get returns one bid, or repository.ErrBidNotFound when the caller is party
// to nothing about it.
func (s *Service) Get(ctx context.Context, userID, bidID string) (*domain.Bid, error) {
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}
	return s.repo.FindBid(ctx, s.repo.Pool(), bidID, userID, nullable(lapakID))
}

// List returns the caller's page of bids. The persona scoping is the server's:
// a customer sees the bids they created, a lapak sees open manual bids plus
// the automatic bids they were matched to, and no query parameter chooses
// whose bids come back.
func (s *Service) List(ctx context.Context, userID, mode, status string, page, limit int) ([]*domain.Bid, int64, error) {
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.ListBids(ctx, s.repo.Pool(), userID, nullable(lapakID),
		nullable(mode), nullable(status), page, limit)
}

// Confirm is the customer's half of the automatic flow: proposed →
// customer_confirmed. No money moves — the fee was charged at creation.
func (s *Service) Confirm(ctx context.Context, userID, bidID string) (*domain.Bid, error) {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin confirm transaction", logger.Err(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lb, err := s.repo.LockBid(ctx, tx, bidID)
	if err != nil {
		return nil, err
	}
	if lb.CustomerUserID != userID {
		return nil, s.refuse(ctx, tx, bidID, userID, ErrNotCustomer)
	}
	if lb.Status != domain.StatusProposed {
		return nil, ErrWrongStatus
	}

	moved, err := s.repo.AdvanceBid(ctx, tx, bidID, domain.StatusProposed, domain.StatusCustomerConfirmed)
	if err != nil {
		return nil, err
	}
	if moved != 1 {
		return nil, ErrBidChanged
	}

	bid, err := s.repo.FindBid(ctx, tx, bidID, userID, nil)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit confirm transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Automatic bid confirmed by customer",
		logger.String("bid_id", bidID), logger.String("user_id", userID))
	return bid, nil
}

// Accept is the matched lapak's half: customer_confirmed → accepted, and the
// tracked order is created in the SAME transaction — one item priced at the
// bid's budget_idr, status pending_payment, orders.bid_id set — with the
// customer↔lapak chat thread opened alongside it.
//
// The thread is opened here, BEFORE any payment (amendment v1.0.2's bid half):
// the pair have to be able to message about a job the customer has not paid
// for yet.
//
// The bid's row lock plus the re-read of its status inside the transaction is
// what makes ten concurrent accepts produce exactly one order — the same shape
// as the order module's completeOrder. orders_bid_id_key is the backstop that
// proves it, not the mechanism.
func (s *Service) Accept(ctx context.Context, userID, bidID string) (*domain.Bid, error) {
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin accept transaction", logger.Err(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lb, err := s.repo.LockBid(ctx, tx, bidID)
	if err != nil {
		return nil, err
	}

	// The 403/404 fork, and it is NOT the same one /confirm uses. Accepting is
	// a lapak's verb, so every lapak has standing to be told "you are not the
	// matched lapak" (403) — that is the contract's own wording. So does the
	// bid's customer, who can already read the row. A caller who is neither
	// learns nothing: 404.
	if lapakID == "" || lb.MatchedLapakID == nil || *lb.MatchedLapakID != lapakID {
		if lapakID != "" || lb.CustomerUserID == userID {
			return nil, ErrNotMatchedLapak
		}
		return nil, repository.ErrBidNotFound
	}
	if lb.Status != domain.StatusCustomerConfirmed {
		return nil, ErrWrongStatus
	}

	orderID, err := s.createTrackedOrder(ctx, tx, domain.OrderSourceAuto,
		lb.CustomerUserID, lapakID, bidID, itemName(lb), lb.BudgetIDR)
	if err != nil {
		return nil, err
	}

	moved, err := s.repo.AdvanceBid(ctx, tx, bidID, domain.StatusCustomerConfirmed, domain.StatusAccepted)
	if err != nil {
		return nil, err
	}
	if moved != 1 {
		logger.Error("Accept aborted: bid left customer_confirmed mid-transaction",
			logger.String("bid_id", bidID), logger.Int64("rows_affected", moved))
		return nil, ErrBidChanged
	}

	bid, err := s.repo.FindBid(ctx, tx, bidID, userID, &lapakID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit accept transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Automatic bid accepted; tracked order created",
		logger.String("bid_id", bidID),
		logger.String("order_id", orderID),
		logger.String("lapak_id", lapakID),
		logger.Int64("amount_idr", lb.BudgetIDR))
	return bid, nil
}

// PlaceOffer is BE-09's worker half. One offer per lapak per bid: posting
// again REPLACES the amount and message, which is why the bool comes back —
// true means a row was created (201), false means one was replaced (200).
// Neither is a duplicate and neither is a rejection.
func (s *Service) PlaceOffer(ctx context.Context, userID, bidID string, amountIDR int64, message string) (*domain.Offer, bool, error) {
	// Only lapaks offer. Checked before the bid is looked up: the contract's
	// 403 here is about the caller's persona, not their relationship to a
	// particular bid.
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, false, err
	}
	if lapakID == "" {
		return nil, false, ErrNotLapak
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin offer transaction", logger.Err(err))
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The bid's row lock keeps an offer from landing on a bid that is being
	// awarded in another transaction at this instant.
	lb, err := s.repo.LockBid(ctx, tx, bidID)
	if err != nil {
		return nil, false, err
	}
	if lb.Mode != domain.ModeManual || lb.Status != domain.StatusOpen {
		return nil, false, ErrWrongStatus
	}

	offer, created, err := s.repo.UpsertOffer(ctx, tx, bidID, lapakID, amountIDR, message)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit offer transaction", logger.Err(err))
		return nil, false, err
	}

	logger.Info("Bid offer placed",
		logger.String("bid_id", bidID),
		logger.String("offer_id", offer.ID),
		logger.String("lapak_id", lapakID),
		logger.Int64("amount_idr", amountIDR),
		logger.Bool("created", created))
	return offer, created, nil
}

// ListOffers is GET /bids/{id}/offers. The bid's customer sees every offer;
// each lapak sees their own and no one else's. Anybody who can see nothing
// about the bid gets 404 from the read itself.
func (s *Service) ListOffers(ctx context.Context, userID, bidID string) ([]*domain.Offer, error) {
	lapakID, err := s.repo.LapakIDForUser(ctx, s.repo.Pool(), userID)
	if err != nil {
		return nil, err
	}

	bid, err := s.repo.FindBid(ctx, s.repo.Pool(), bidID, userID, nullable(lapakID))
	if err != nil {
		return nil, err
	}

	// A lapak's scope is their own offer. The customer's is the whole list —
	// they are the one choosing between them.
	scope := nullable(lapakID)
	if bid.Customer.ID == userID {
		scope = nil
	}
	return s.repo.ListOffers(ctx, s.repo.Pool(), bidID, scope)
}

// Award is BE-09's customer half and the module's second money path. ONE
// transaction covers all of it: the manual fee with its `platform_fee` ledger
// row, the bid → awarded, the winning offer → awarded, the tracked order
// priced at the OFFER amount (not the bid's budget), and the chat thread.
//
// Order of operations, and why:
//
//  1. lock the bid row and re-read its status — the double-award guard, and
//     the reason two concurrent awards produce one order and one fee;
//  2. ownership and the offer, so a 402 is never raised for a request that was
//     going to fail anyway;
//  3. lock the wallet, then compare balance to fee: 402 here writes nothing
//     and the bid stays `open`;
//  4. the writes;
//  5. commit.
func (s *Service) Award(ctx context.Context, userID, bidID, offerID string) (*domain.Bid, error) {
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin award transaction", logger.Err(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1.
	lb, err := s.repo.LockBid(ctx, tx, bidID)
	if err != nil {
		return nil, err
	}

	// 2. Awarding is the customer's verb: anyone else who can see the bid gets
	// 403, and anyone who cannot gets 404.
	if lb.CustomerUserID != userID {
		return nil, s.refuse(ctx, tx, bidID, userID, ErrNotCustomer)
	}
	if lb.Mode != domain.ModeManual || lb.Status != domain.StatusOpen {
		return nil, ErrWrongStatus
	}

	offer, err := s.repo.FindOffer(ctx, tx, bidID, offerID)
	if err != nil {
		return nil, err
	}

	fee, err := s.repo.FeeIDR(ctx, tx, domain.ConfigKeyManualFee)
	if err != nil {
		return nil, err
	}

	// 3.
	balance, err := s.repo.LockWalletForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if balance < fee {
		logger.Warn("Award refused: insufficient balance for the platform fee",
			logger.String("bid_id", bidID),
			logger.String("user_id", userID),
			logger.Int64("balance_idr", balance),
			logger.Int64("fee_idr", fee))
		return nil, ErrInsufficientBalance
	}

	// 4.
	afterFee, err := s.repo.MoveWallet(ctx, tx, userID, -fee)
	if err != nil {
		return nil, err
	}
	if err := s.repo.InsertLedgerEntry(ctx, tx, userID, domain.LedgerTypePlatformFee,
		-fee, afterFee, bidID, "Manual bid award fee"); err != nil {
		return nil, err
	}

	awarded, err := s.repo.MarkOfferAwarded(ctx, tx, offer.ID)
	if err != nil {
		return nil, err
	}
	if awarded != 1 {
		logger.Error("Award aborted: offer was no longer pending",
			logger.String("bid_id", bidID), logger.String("offer_id", offer.ID))
		return nil, ErrWrongStatus
	}

	orderID, err := s.createTrackedOrder(ctx, tx, domain.OrderSourceManual,
		userID, offer.Lapak.ID, bidID, itemName(lb), offer.AmountIDR)
	if err != nil {
		return nil, err
	}

	moved, err := s.repo.AdvanceBid(ctx, tx, bidID, domain.StatusOpen, domain.StatusAwarded)
	if err != nil {
		return nil, err
	}
	if moved != 1 {
		logger.Error("Award aborted: bid left open mid-transaction",
			logger.String("bid_id", bidID), logger.Int64("rows_affected", moved))
		return nil, ErrBidChanged
	}
	if err := s.repo.SetBidFeePaid(ctx, tx, bidID, fee); err != nil {
		return nil, err
	}

	bid, err := s.repo.FindBid(ctx, tx, bidID, userID, nil)
	if err != nil {
		return nil, err
	}

	// 5.
	if err := tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit award transaction", logger.Err(err))
		return nil, err
	}

	logger.Info("Manual bid awarded; fee charged and tracked order created",
		logger.String("bid_id", bidID),
		logger.String("offer_id", offer.ID),
		logger.String("order_id", orderID),
		logger.String("lapak_id", offer.Lapak.ID),
		logger.Int64("fee_idr", fee),
		logger.Int64("balance_after_idr", afterFee),
		logger.Int64("amount_idr", offer.AmountIDR))
	return bid, nil
}

// createTrackedOrder is the one place a bid turns into an order, shared by
// /accept and /award so the two cannot drift about what a bid-produced order
// is: one unpaid item, pending_payment, bid_id set — and the chat thread
// opened at the same moment, before any money has moved.
//
// The two callers differ in exactly three arguments: the source, the lapak,
// and the price. That is the whole difference between the modes at this point.
func (s *Service) createTrackedOrder(ctx context.Context, tx repository.DB, source, customerUserID, lapakID, bidID, name string, priceIDR int64) (string, error) {
	orderID, err := s.repo.InsertBidOrder(ctx, tx, source, customerUserID, lapakID, bidID)
	if err != nil {
		return "", err
	}
	if err := s.repo.InsertBidOrderItem(ctx, tx, orderID, name, priceIDR); err != nil {
		return "", err
	}
	// Amendment v1.0.2, bid half: the thread exists from this moment, while
	// the order is still pending_payment.
	if err := s.repo.EnsureChatThread(ctx, tx, orderID); err != nil {
		return "", err
	}
	return orderID, nil
}

// refuse decides what a failed ownership test means, and is the one place the
// 403/404 fork lives for the customer verbs (/confirm, /award) so they cannot
// answer it two different ways.
//
// It runs only on the failure path. The question it asks is "can this caller
// see the bid at all?" — if yes, the action simply is not theirs (403); if no,
// telling them the bid exists would itself be the leak (404).
//
// /accept does NOT use this: its 403 is about being a lapak, not about being
// party to the bid. See the comment there.
func (s *Service) refuse(ctx context.Context, db repository.DB, bidID, userID string, forbidden error) error {
	lapakID, err := s.repo.LapakIDForUser(ctx, db, userID)
	if err != nil {
		return err
	}
	visible, err := s.repo.CanSeeBid(ctx, db, bidID, userID, nullable(lapakID))
	if err != nil {
		return err
	}
	if visible {
		return forbidden
	}
	return repository.ErrBidNotFound
}

// itemName is what the tracked order's single line is called. An automatic bid
// carries no title (the contract only requires one for manual), so the
// category name is the fallback — an order item with a blank name would be
// unreadable in every list that shows it.
func itemName(lb *repository.LockedBid) string {
	if lb.Title != "" {
		return lb.Title
	}
	return lb.CategoryName
}

// nullable turns "no value" into a SQL NULL, which is what makes the
// visibility and filter predicates behave: a customer's NULL lapak id can
// never equal a real one, and a NULL filter disables its own arm entirely.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
