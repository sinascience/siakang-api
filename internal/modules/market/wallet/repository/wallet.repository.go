package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/wallet/domain"
	"siakang-api/pkg/logger"
)

// ErrWalletNotFound means this user has no market.wallets row — reachable
// via plain signup, which creates a core.users row with no market
// provisioning behind it. The contract has no 404 for GET /wallet, so the
// handler maps this to 200/balance_idr:0 rather than an error status; it
// stays a distinct error here so that mapping is a deliberate handler
// decision, logged as a warning, not a silently absorbed one.
var ErrWalletNotFound = errors.New("wallet not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetWallet returns userID's wallet. userID must come from the JWT — this is
// the entire authorization model for the market domain (no CompanyContext,
// no RequirePermission), so there is deliberately no other way to select a
// row here than by the caller's own id.
func (r *Repository) GetWallet(ctx context.Context, userID string) (*domain.Wallet, error) {
	const query = `SELECT user_id, balance_idr FROM market.wallets WHERE user_id = $1`

	var w domain.Wallet
	err := r.db.QueryRow(ctx, query, userID).Scan(&w.UserID, &w.BalanceIDR)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWalletNotFound
		}
		logger.Error("Failed to get wallet", logger.Err(err))
		return nil, err
	}
	return &w, nil
}

// GetLedger returns userID's ledger entries newest-first, page/limit
// already clamped by the handler, plus the total row count for pagination.
// A user with no entries gets an empty, non-nil slice and total 0 — never
// an error.
func (r *Repository) GetLedger(ctx context.Context, userID string, page, limit int) ([]domain.LedgerEntry, int64, error) {
	const countQuery = `SELECT COUNT(*) FROM market.ledger_entries WHERE user_id = $1`
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		logger.Error("Failed to count ledger entries", logger.Err(err))
		return nil, 0, err
	}

	// id DESC is a tiebreak, not decoration: created_at defaults to NOW(),
	// which in Postgres is the TRANSACTION timestamp, so two rows written in
	// the same transaction (e.g. an auto-bid's fee + its no_match refund)
	// share one created_at. Without a tiebreak their relative order is
	// undefined, and a row can appear on two LIMIT/OFFSET pages or on none.
	const dataQuery = `
		SELECT id, type, amount_idr, balance_after_idr, order_id, bid_id, note, created_at
		FROM market.ledger_entries
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	offset := (page - 1) * limit
	rows, err := r.db.Query(ctx, dataQuery, userID, limit, offset)
	if err != nil {
		logger.Error("Failed to list ledger entries", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	entries := make([]domain.LedgerEntry, 0)
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(&e.ID, &e.Type, &e.AmountIDR, &e.BalanceAfterIDR, &e.OrderID, &e.BidID, &e.Note, &e.CreatedAt); err != nil {
			logger.Error("Failed to scan ledger entry", logger.Err(err))
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate ledger entries", logger.Err(err))
		return nil, 0, err
	}

	return entries, total, nil
}
