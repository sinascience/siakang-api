package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/bid/domain"
	"siakang-api/pkg/logger"
)

var (
	// ErrBidNotFound means "no such bid, OR the caller is party to nothing
	// about it". The two are deliberately indistinguishable on the read path:
	// the visibility test lives in the WHERE clause, so a stranger gets the
	// same empty result as a bad id and the handler answers 404. Where the
	// contract lists a 403 the service raises it explicitly instead — see
	// Service.refuse.
	ErrBidNotFound = errors.New("bid not found")

	// ErrOfferNotFound covers an unknown offer id, and an offer id that
	// belongs to a different bid than the path names.
	ErrOfferNotFound = errors.New("offer not found")

	// ErrCategoryNotFound is a 422 on POST /bids, not a 404: category_id is a
	// body field, so an unknown one is a bad request body.
	ErrCategoryNotFound = errors.New("bid category not found")
)

// DB is the subset of pgx that both *pgxpool.Pool and pgx.Tx satisfy, so the
// same method runs on the pool for a plain read and on the transaction for the
// money paths — without a second set of tx-only methods that could drift.
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
// Persona and config
// =====================================================================

// LapakIDForUser returns the caller's lapak profile id, or "" when the caller
// is a customer. One query answers every persona question this module asks:
// whether a bid may be created (lapaks may not), whether an offer may be
// placed (only lapaks may), and which bids the caller's list is scoped to.
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

// FeeIDR reads one platform fee from market.config. Never a Go constant: the
// row is the source of truth and QA edits it to prove the fee is configurable,
// so a hard-coded 2500 would be a lie the tests could not catch.
//
// A missing key is an error rather than a zero fee — silently charging nothing
// is a money bug, not a default.
func (r *Repository) FeeIDR(ctx context.Context, db DB, key string) (int64, error) {
	const query = `SELECT value FROM market.config WHERE key = $1`

	var value int64
	if err := db.QueryRow(ctx, query, key).Scan(&value); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf("market.config missing required key %q", key)
		}
		logger.Error("Failed to read platform fee", logger.String("key", key), logger.Err(err))
		return 0, err
	}
	return value, nil
}

// =====================================================================
// Categories
// =====================================================================

// ListCategories backs GET /market/v1/bid-categories. No pagination: the set
// is admin-predeclared and seeded, and there is no admin UI in sprint 1.
func (r *Repository) ListCategories(ctx context.Context, db DB) ([]domain.Category, error) {
	const query = `
		SELECT id, name, slug
		FROM market.bid_categories
		WHERE deleted_at IS NULL
		ORDER BY name, id
	`
	rows, err := db.Query(ctx, query)
	if err != nil {
		logger.Error("Failed to list bid categories", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	categories := make([]domain.Category, 0)
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug); err != nil {
			logger.Error("Failed to scan bid category", logger.Err(err))
			return nil, err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate bid categories", logger.Err(err))
		return nil, err
	}
	return categories, nil
}

// FindCategory resolves the bid's category before anything is written. It is
// also the item-name fallback for an automatic bid, which carries no title.
func (r *Repository) FindCategory(ctx context.Context, db DB, id string) (*domain.Category, error) {
	const query = `
		SELECT id, name, slug
		FROM market.bid_categories
		WHERE id = $1 AND deleted_at IS NULL
	`
	var c domain.Category
	err := db.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.Slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		logger.Error("Failed to find bid category", logger.Err(err))
		return nil, err
	}
	return &c, nil
}

// =====================================================================
// Wallet and ledger — the money primitives, same shape as the order
// module's so the two cannot disagree about what a charge is.
// =====================================================================

// LockWalletForUpdate takes the row lock that serializes every charge against
// this wallet, and returns the balance as of the lock. Read the fee and
// compare AFTER this call, never before: under READ COMMITTED the next
// statement sees whatever a competing charge already committed.
//
// A user with no wallet row has no balance and no lock — but zero cannot cover
// a positive fee, so that path always ends in 402 before anything is written.
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

// MoveWallet applies a signed delta to the locked wallet and returns what the
// balance became — the value the ledger row must record as balance_after_idr.
//
// One method for the debit and the refund, because they are the same statement
// with the sign flipped and a second copy would eventually drift about money.
// The arithmetic happens in SQL on the locked row, so a balance can never be
// built from a stale read.
func (r *Repository) MoveWallet(ctx context.Context, db DB, userID string, deltaIDR int64) (int64, error) {
	const query = `
		UPDATE market.wallets
		SET balance_idr = balance_idr + $1, updated_at = NOW()
		WHERE user_id = $2
		RETURNING balance_idr
	`
	var balance int64
	if err := db.QueryRow(ctx, query, deltaIDR, userID).Scan(&balance); err != nil {
		logger.Error("Failed to move wallet balance", logger.Err(err))
		return 0, err
	}
	return balance, nil
}

// InsertLedgerEntry appends the money journal row against a BID (the order
// module's twin writes order_id instead). amountIDR is signed: negative for
// the platform fee, positive for the refund.
//
// Append-only, and created_at defaults to NOW() — the TRANSACTION timestamp,
// so a no_match's platform_fee and refund rows carry a byte-identical
// created_at. Any query ordering by created_at needs `, id DESC` to be
// deterministic; the wallet ledger endpoint already has it.
func (r *Repository) InsertLedgerEntry(ctx context.Context, db DB, userID, entryType string, amountIDR, balanceAfterIDR int64, bidID, note string) error {
	const query = `
		INSERT INTO market.ledger_entries (user_id, type, amount_idr, balance_after_idr, bid_id, note)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if _, err := db.Exec(ctx, query, userID, entryType, amountIDR, balanceAfterIDR, bidID, note); err != nil {
		logger.Error("Failed to insert ledger entry", logger.Err(err))
		return err
	}
	return nil
}

// =====================================================================
// Bids — write
// =====================================================================

// InsertBid writes the bid row. For an automatic bid the caller has ALREADY
// decided the fee is affordable (the wallet is locked and compared first), so
// feePaidIDR arrives as the fee about to be charged in the next statement.
//
// lat/lng are pointers: NULL for a manual bid, and chk_bids_auto_has_origin
// rejects an automatic one without them.
func (r *Repository) InsertBid(ctx context.Context, db DB, mode, status, categoryID, customerUserID, title, description string, budgetIDR int64, lat, lng *float64, feePaidIDR int64) (string, error) {
	const query = `
		INSERT INTO market.bids
			(mode, status, category_id, customer_user_id, title, description,
			 budget_idr, lat, lng, fee_paid_idr)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	var id string
	err := db.QueryRow(ctx, query, mode, status, categoryID, customerUserID, title,
		description, budgetIDR, lat, lng, feePaidIDR).Scan(&id)
	if err != nil {
		logger.Error("Failed to insert bid", logger.Err(err))
		return "", err
	}
	return id, nil
}

// FindNearestLapak is the whole of automatic matching, and it runs AFTER the
// fee has been charged — that ordering is criterion BE-08.1, not an
// implementation detail.
//
// Candidates are lapaks that are in the bid's category (market.lapak_categories,
// the join table that makes "nearest lapak IN THIS CATEGORY" answerable at
// all), available, and not soft-deleted. Ordered by haversine distance
// ascending with rating descending as the tiebreak — in that order, which is
// what makes the seed fixture falsifiable: sari is rated higher than joko and
// must still lose on distance, and agus is nearest and best rated and must
// still lose on is_available.
//
// No maps API in sprint 1: the haversine is arithmetic in SQL over the seeded
// coordinates, with 6371 km as the earth's mean radius.
//
// Returns (nil, nil) when the category has no available lapak — the no_match
// case, which the caller answers with a refund in this same transaction.
func (r *Repository) FindNearestLapak(ctx context.Context, db DB, categoryID string, lat, lng float64) (*domain.Match, error) {
	const query = `
		SELECT l.id, l.name, l.rating,
		       6371 * 2 * asin(sqrt(
		           power(sin(radians(l.lat - $2) / 2), 2)
		         + cos(radians($2)) * cos(radians(l.lat))
		         * power(sin(radians(l.lng - $3) / 2), 2)
		       )) AS distance_km
		FROM market.lapak_profiles l
		JOIN market.lapak_categories lc
		  ON lc.lapak_id = l.id AND lc.category_id = $1
		WHERE l.deleted_at IS NULL AND l.is_available = TRUE
		ORDER BY distance_km ASC, l.rating DESC, l.id
		LIMIT 1
	`
	var m domain.Match
	err := db.QueryRow(ctx, query, categoryID, lat, lng).
		Scan(&m.Lapak.ID, &m.Lapak.Name, &m.Lapak.Rating, &m.DistanceKM)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to match nearest lapak", logger.Err(err))
		return nil, err
	}
	return &m, nil
}

// MarkBidProposed records the match on the bid so GET /bids/{id} can report
// matched_lapak and matched_distance_km without recomputing a distance that
// would drift if a lapak ever moved.
func (r *Repository) MarkBidProposed(ctx context.Context, db DB, bidID, lapakID string, distanceKM float64) error {
	const query = `
		UPDATE market.bids
		SET status = $2, matched_lapak_id = $3, matched_distance_km = $4, updated_at = NOW()
		WHERE id = $1
	`
	if _, err := db.Exec(ctx, query, bidID, domain.StatusProposed, lapakID, distanceKM); err != nil {
		logger.Error("Failed to mark bid proposed", logger.Err(err))
		return err
	}
	return nil
}

// SetBidFeePaid rewrites the fee snapshot on the bid. Called with 0 by the
// no_match refund — the fee came back, so the bid must not keep claiming it
// was charged — and with the manual fee at award.
func (r *Repository) SetBidFeePaid(ctx context.Context, db DB, bidID string, feeIDR int64) error {
	const query = `UPDATE market.bids SET fee_paid_idr = $2, updated_at = NOW() WHERE id = $1`

	if _, err := db.Exec(ctx, query, bidID, feeIDR); err != nil {
		logger.Error("Failed to set bid fee_paid_idr", logger.Err(err))
		return err
	}
	return nil
}

// LockedBid is what LockBid returns: the fields every transition needs, read
// as of the row lock.
type LockedBid struct {
	ID             string
	Mode           string
	Status         string
	CustomerUserID string
	MatchedLapakID *string
	BudgetIDR      int64
	Title          string
	CategoryName   string
}

// LockBid takes the bid's row lock and re-reads its status inside the caller's
// transaction. This is the double-accept and double-award guard, and the same
// shape the order module's completeOrder uses for its double-payout guard: ten
// concurrent /accept calls all queue here, and the nine that lose find a
// status that no longer permits the transition.
//
// The unique index on orders.bid_id is the backstop that proves this lock
// works, not the mechanism relied on.
func (r *Repository) LockBid(ctx context.Context, db DB, bidID string) (*LockedBid, error) {
	const query = `
		SELECT b.id, b.mode, b.status, b.customer_user_id, b.matched_lapak_id,
		       b.budget_idr, b.title, c.name
		FROM market.bids b
		JOIN market.bid_categories c ON c.id = b.category_id
		WHERE b.id = $1
		FOR UPDATE OF b
	`
	var lb LockedBid
	err := db.QueryRow(ctx, query, bidID).Scan(&lb.ID, &lb.Mode, &lb.Status,
		&lb.CustomerUserID, &lb.MatchedLapakID, &lb.BudgetIDR, &lb.Title, &lb.CategoryName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBidNotFound
		}
		logger.Error("Failed to lock bid", logger.Err(err))
		return nil, err
	}
	return &lb, nil
}

// AdvanceBid moves the bid from one status to another, guarded on the status
// it expects to find. Returns the rows affected: anything but 1 means the row
// moved between the lock and this statement, which the service treats as a
// reason to roll back rather than to write a second order.
func (r *Repository) AdvanceBid(ctx context.Context, db DB, bidID, from, to string) (int64, error) {
	const query = `
		UPDATE market.bids
		SET status = $3, updated_at = NOW()
		WHERE id = $1 AND status = $2
	`
	tag, err := db.Exec(ctx, query, bidID, from, to)
	if err != nil {
		logger.Error("Failed to advance bid status", logger.Err(err))
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// =====================================================================
// Bids — read
// =====================================================================

// bidColumns and bidJoins are shared by the detail and list queries so the two
// cannot drift.
//
// offer_count, accepted_offer_id and order_id are derived here rather than
// stored on the bid — the schema's rule, because a stored copy is a copy that
// can drift. order_id is the reverse of orders.bid_id: the pair needs one
// foreign key, not two pointing at each other.
const bidColumns = `
	b.id, b.mode, b.status,
	c.id, c.name, c.slug,
	b.customer_user_id, COALESCE(cu.full_name, ''),
	b.title, b.description, b.budget_idr, b.lat, b.lng, b.fee_paid_idr,
	ml.id, ml.name, ml.rating, b.matched_distance_km,
	(SELECT COUNT(*) FROM market.bid_offers o WHERE o.bid_id = b.id),
	(SELECT o.id FROM market.bid_offers o WHERE o.bid_id = b.id AND o.status = 'awarded'),
	ord.id,
	b.created_at
`

const bidJoins = `
	FROM market.bids b
	JOIN market.bid_categories c ON c.id = b.category_id
	JOIN core.users cu ON cu.id = b.customer_user_id
	LEFT JOIN market.lapak_profiles ml ON ml.id = b.matched_lapak_id
	LEFT JOIN market.orders ord ON ord.bid_id = b.id
`

// visibility is this module's entire read authorization, expressed as a WHERE
// fragment: $1 is the caller's user id, $2 their lapak profile id or NULL.
//
// A caller may see a bid when they are its customer, when they are the lapak
// it matched, when they are any lapak and it is an open manual bid (that list
// IS the lapak's flow-C inbox), or when they are the lapak that placed an
// offer on it — so the lapak who wins an award does not lose sight of the bid
// the moment it stops being open.
//
// A customer's NULL lapak id can never equal a real one, so the lapak arms are
// unreachable for them by construction rather than by a Go check.
const visibility = `(
	    b.customer_user_id = $1
	 OR b.matched_lapak_id = $2
	 OR ($2 IS NOT NULL AND b.mode = 'manual' AND b.status = 'open')
	 OR ($2 IS NOT NULL AND EXISTS (
	        SELECT 1 FROM market.bid_offers o WHERE o.bid_id = b.id AND o.lapak_id = $2))
	)`

func scanBid(row pgx.Row) (*domain.Bid, error) {
	var (
		b                  domain.Bid
		lapakID, lapakName *string
		lapakRating        *float64
	)
	err := row.Scan(
		&b.ID, &b.Mode, &b.Status,
		&b.Category.ID, &b.Category.Name, &b.Category.Slug,
		&b.Customer.ID, &b.Customer.FullName,
		&b.Title, &b.Description, &b.BudgetIDR, &b.Lat, &b.Lng, &b.FeePaidIDR,
		&lapakID, &lapakName, &lapakRating, &b.MatchedDistanceKM,
		&b.OfferCount, &b.AcceptedOfferID, &b.OrderID,
		&b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	// The LEFT JOIN gives three NULLs or three values together, so one test
	// covers the summary.
	if lapakID != nil {
		b.MatchedLapak = &domain.LapakSummary{ID: *lapakID, Name: *lapakName, Rating: *lapakRating}
	}
	return &b, nil
}

// FindBid loads one bid, or ErrBidNotFound when the caller can see nothing
// about it. The visibility test is in the query rather than a Go check after
// the fetch, so no code path ever holds the row and then decides not to show
// it.
func (r *Repository) FindBid(ctx context.Context, db DB, bidID, userID string, lapakID *string) (*domain.Bid, error) {
	query := `SELECT ` + bidColumns + bidJoins + ` WHERE ` + visibility + ` AND b.id = $3`

	bid, err := scanBid(db.QueryRow(ctx, query, userID, lapakID, bidID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBidNotFound
		}
		logger.Error("Failed to find bid", logger.Err(err))
		return nil, err
	}
	return bid, nil
}

// CanSeeBid answers the 403-vs-404 fork on the failure path only: a caller who
// can see the bid but used the wrong verb gets 403, and one who can see
// nothing gets 404. It runs the same visibility fragment the reads use, so the
// two answers cannot disagree about who is party to a bid.
func (r *Repository) CanSeeBid(ctx context.Context, db DB, bidID, userID string, lapakID *string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM market.bids b WHERE ` + visibility + ` AND b.id = $3)`

	var visible bool
	if err := db.QueryRow(ctx, query, userID, lapakID, bidID).Scan(&visible); err != nil {
		logger.Error("Failed to test bid visibility", logger.Err(err))
		return false, err
	}
	return visible, nil
}

// BidExists distinguishes "no such bid" from "not yours" without applying the
// visibility test — used only where an endpoint's 403 is about the caller's
// persona rather than their relationship to the bid.
func (r *Repository) BidExists(ctx context.Context, db DB, bidID string) (bool, error) {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM market.bids WHERE id = $1)`, bidID).Scan(&exists); err != nil {
		logger.Error("Failed to test bid existence", logger.Err(err))
		return false, err
	}
	return exists, nil
}

// ListBids returns one page of the bids relevant to the caller, newest first.
//
// The persona scoping is the server's and reuses the same visibility fragment
// as the detail read: a customer sees the bids they created, a lapak sees open
// manual bids plus the automatic bids they were matched to. There is no query
// parameter that chooses whose bids come back.
//
// ORDER BY carries `, b.id DESC` as a tiebreak, not decoration: created_at
// defaults to NOW(), the TRANSACTION timestamp, so rows written together share
// it byte-for-byte and their relative order would otherwise be undefined.
func (r *Repository) ListBids(ctx context.Context, db DB, userID string, lapakID, mode, status *string, page, limit int) ([]*domain.Bid, int64, error) {
	// $1 userID and $2 lapakID belong to the visibility fragment; the two
	// filters are *string so a nil disables its own arm in SQL rather than by
	// concatenating a different query string.
	const filters = ` AND ($3::text IS NULL OR b.mode = $3) AND ($4::text IS NULL OR b.status = $4)`

	countQuery := `SELECT COUNT(*) FROM market.bids b WHERE ` + visibility + filters
	var total int64
	if err := db.QueryRow(ctx, countQuery, userID, lapakID, mode, status).Scan(&total); err != nil {
		logger.Error("Failed to count bids", logger.Err(err))
		return nil, 0, err
	}
	if total == 0 {
		return []*domain.Bid{}, 0, nil
	}

	dataQuery := `SELECT ` + bidColumns + bidJoins + ` WHERE ` + visibility + filters +
		` ORDER BY b.created_at DESC, b.id DESC LIMIT $5 OFFSET $6`

	rows, err := db.Query(ctx, dataQuery, userID, lapakID, mode, status, limit, (page-1)*limit)
	if err != nil {
		logger.Error("Failed to list bids", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	bids := make([]*domain.Bid, 0)
	for rows.Next() {
		b, err := scanBid(rows)
		if err != nil {
			logger.Error("Failed to scan bid", logger.Err(err))
			return nil, 0, err
		}
		bids = append(bids, b)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate bids", logger.Err(err))
		return nil, 0, err
	}
	return bids, total, nil
}

// =====================================================================
// Offers
// =====================================================================

const offerColumns = `
	o.id, o.bid_id, l.id, l.name, l.rating,
	o.amount_idr, o.message, o.status, o.created_at
`

func scanOffer(row pgx.Row) (*domain.Offer, error) {
	var o domain.Offer
	err := row.Scan(&o.ID, &o.BidID, &o.Lapak.ID, &o.Lapak.Name, &o.Lapak.Rating,
		&o.AmountIDR, &o.Message, &o.Status, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// UpsertOffer places or REPLACES this lapak's offer on the bid — criterion
// BE-09.4, and the schema's own rule: bid_offers_bid_lapak_key makes one row
// per lapak per bid, so posting again is an UPDATE, not a duplicate and not a
// rejection.
//
// The bool reports whether a row was created, which is the 201-vs-200 fork.
// It is decided by comparing the returned timestamps rather than by peeking at
// xmax: on an insert both default to NOW(), and the replace path sets
// updated_at to a later transaction's NOW(). No system columns, no surprises.
//
// A replace resets status to `pending`: an offer that was rejected and then
// re-priced is a live offer again, and the partial unique index still allows
// only one `awarded` offer per bid.
func (r *Repository) UpsertOffer(ctx context.Context, db DB, bidID, lapakID string, amountIDR int64, message string) (*domain.Offer, bool, error) {
	const query = `
		WITH upserted AS (
			INSERT INTO market.bid_offers (bid_id, lapak_id, amount_idr, message)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT ON CONSTRAINT bid_offers_bid_lapak_key DO UPDATE
				SET amount_idr = EXCLUDED.amount_idr,
				    message    = EXCLUDED.message,
				    status     = 'pending',
				    updated_at = NOW()
			RETURNING id, bid_id, lapak_id, amount_idr, message, status, created_at, updated_at
		)
		SELECT ` + offerColumns + `, o.updated_at
		FROM upserted o
		JOIN market.lapak_profiles l ON l.id = o.lapak_id
	`
	var (
		o                  domain.Offer
		createdAt, updated time.Time
	)
	err := db.QueryRow(ctx, query, bidID, lapakID, amountIDR, message).
		Scan(&o.ID, &o.BidID, &o.Lapak.ID, &o.Lapak.Name, &o.Lapak.Rating,
			&o.AmountIDR, &o.Message, &o.Status, &createdAt, &updated)
	if err != nil {
		logger.Error("Failed to upsert bid offer", logger.Err(err))
		return nil, false, err
	}
	o.CreatedAt = createdAt
	return &o, createdAt.Equal(updated), nil
}

// ListOffers returns a bid's offers, cheapest first — the order the schema's
// idx_bid_offers_bid_amount exists for.
//
// onlyLapakID narrows the list to one lapak's own offer: the contract makes
// the full list the customer's and gives each lapak sight of their own offer
// only, so the scope is a query argument rather than a filter applied after
// the rows are already in memory.
func (r *Repository) ListOffers(ctx context.Context, db DB, bidID string, onlyLapakID *string) ([]*domain.Offer, error) {
	query := `
		SELECT ` + offerColumns + `
		FROM market.bid_offers o
		JOIN market.lapak_profiles l ON l.id = o.lapak_id
		WHERE o.bid_id = $1 AND ($2::uuid IS NULL OR o.lapak_id = $2)
		ORDER BY o.amount_idr ASC, o.id
	`
	rows, err := db.Query(ctx, query, bidID, onlyLapakID)
	if err != nil {
		logger.Error("Failed to list bid offers", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	offers := make([]*domain.Offer, 0)
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			logger.Error("Failed to scan bid offer", logger.Err(err))
			return nil, err
		}
		offers = append(offers, o)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate bid offers", logger.Err(err))
		return nil, err
	}
	return offers, nil
}

// FindOffer loads one offer, scoped to the bid the path names. An offer id
// from a different bid is ErrOfferNotFound rather than a cross-bid award.
func (r *Repository) FindOffer(ctx context.Context, db DB, bidID, offerID string) (*domain.Offer, error) {
	query := `
		SELECT ` + offerColumns + `
		FROM market.bid_offers o
		JOIN market.lapak_profiles l ON l.id = o.lapak_id
		WHERE o.id = $1 AND o.bid_id = $2
	`
	// Lapak.ID is market.lapak_profiles.id — the same value orders.lapak_id
	// wants — so the award path needs no second lookup to place the order.
	o, err := scanOffer(db.QueryRow(ctx, query, offerID, bidID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOfferNotFound
		}
		logger.Error("Failed to find bid offer", logger.Err(err))
		return nil, err
	}
	return o, nil
}

// MarkOfferAwarded flips the winning offer. `awarded`, never `accepted`
// (amendment v1.0.1) — chk_bid_offers_status would reject `accepted`, and
// bid_offers_one_awarded_per_bid is the backstop against a second award.
func (r *Repository) MarkOfferAwarded(ctx context.Context, db DB, offerID string) (int64, error) {
	const query = `
		UPDATE market.bid_offers
		SET status = $2, updated_at = NOW()
		WHERE id = $1 AND status = $3
	`
	tag, err := db.Exec(ctx, query, offerID, domain.OfferStatusAwarded, domain.OfferStatusPending)
	if err != nil {
		logger.Error("Failed to mark offer awarded", logger.Err(err))
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// =====================================================================
// The tracked order a bid produces
// =====================================================================

// InsertBidOrder creates the tracked order. chk_orders_bid_source makes bid_id
// legal only on a bid_* source and mandatory on one, so the pair is written
// together or not at all.
//
// orders_bid_id_key — one order per bid — is the unique index that backstops
// the accept and award races. It is not the mechanism: the service holds the
// bid's row lock before it gets here.
func (r *Repository) InsertBidOrder(ctx context.Context, db DB, source, customerUserID, lapakID, bidID string) (string, error) {
	const query = `
		INSERT INTO market.orders (source, status, customer_user_id, lapak_id, bid_id)
		VALUES ($1, 'pending_payment', $2, $3, $4)
		RETURNING id
	`
	var id string
	if err := db.QueryRow(ctx, query, source, customerUserID, lapakID, bidID).Scan(&id); err != nil {
		logger.Error("Failed to insert bid order", logger.Err(err))
		return "", err
	}
	return id, nil
}

// InsertBidOrderItem writes the order's single item. product_id and
// gig_tier_id are both NULL — a bid-produced item is priced from the bid or
// the winning offer, and no catalog row backs it.
func (r *Repository) InsertBidOrderItem(ctx context.Context, db DB, orderID, name string, unitPriceIDR int64) error {
	const query = `
		INSERT INTO market.order_items (order_id, name, unit_price_idr, quantity)
		VALUES ($1, $2, $3, 1)
	`
	if _, err := db.Exec(ctx, query, orderID, name, unitPriceIDR); err != nil {
		logger.Error("Failed to insert bid order item", logger.Err(err))
		return err
	}
	return nil
}

// EnsureChatThread opens the customer↔lapak thread for the order a bid just
// produced — amendment v1.0.2's bid half: the pair must be able to message
// BEFORE the customer pays, so this runs at accept and at award, while the
// order is still pending_payment.
//
// ponytail: this is the same one statement as the order module's
// EnsureChatThread, copied rather than imported. No market submodule imports
// another — each owns its own repository — and a three-line INSERT is a
// smaller price than being the first to couple two modules' data layers.
// Idempotent by the schema's UNIQUE(order_id), which is what lets the order
// module's first payment run it again harmlessly.
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
