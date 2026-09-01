package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/gig/domain"
	"siakang-api/pkg/logger"
)

// ErrGigNotFound means no non-deleted market.gigs row matches the id.
var ErrGigNotFound = errors.New("gig not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// gigSelect joins each gig to its selling lapak so both list and detail
// return the embedded LapakSummary in one query, never a per-row lookup in
// a loop. Tiers are loaded separately, keyed by gig id (see loadTiers).
const gigSelect = `
	SELECT g.id, g.title, g.description, g.image_url,
	       l.id, l.name, l.rating
	FROM market.gigs g
	JOIN market.lapak_profiles l ON l.id = g.lapak_id AND l.deleted_at IS NULL
	WHERE g.deleted_at IS NULL
`

func scanGig(row pgx.Row) (*domain.Gig, error) {
	var g domain.Gig
	err := row.Scan(
		&g.ID, &g.Title, &g.Description, &g.ImageURL,
		&g.LapakID, &g.LapakName, &g.LapakRating,
	)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// loadTiers returns every tier for the given gig ids, price_idr ascending,
// in one query keyed by gig_id — the pattern order/repository uses for
// order items via `= ANY($1::uuid[])`, so a page of N gigs costs one tier
// query total, never one per gig.
func (r *Repository) loadTiers(ctx context.Context, gigIDs []string) (map[string][]domain.GigTier, error) {
	const query = `
		SELECT id, gig_id, name, description, price_idr
		FROM market.gig_tiers
		WHERE gig_id = ANY($1::uuid[]) AND deleted_at IS NULL
		ORDER BY gig_id, price_idr ASC
	`
	rows, err := r.db.Query(ctx, query, gigIDs)
	if err != nil {
		logger.Error("Failed to load gig tiers", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]domain.GigTier, len(gigIDs))
	for rows.Next() {
		var t domain.GigTier
		if err := rows.Scan(&t.ID, &t.GigID, &t.Name, &t.Description, &t.PriceIDR); err != nil {
			logger.Error("Failed to scan gig tier", logger.Err(err))
			return nil, err
		}
		out[t.GigID] = append(out[t.GigID], t)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate gig tiers", logger.Err(err))
		return nil, err
	}
	return out, nil
}

// List returns the catalog page, newest first, optionally filtered by a
// case-insensitive substring match on title, tiers embedded for the whole
// page in one extra query. total reflects the FILTERED set, counted with
// the same WHERE clause the page query uses.
//
// ponytail: q is a plain ILIKE, a sequential scan at seed scale — same as
// products; add pg_trgm + a GIN index if the catalog grows enough for it
// to show up in a query plan.
func (r *Repository) List(ctx context.Context, page, limit int, q string) ([]domain.Gig, int64, error) {
	var args []interface{}
	whereQ := ""
	if q != "" {
		args = append(args, "%"+q+"%")
		whereQ = fmt.Sprintf(" AND g.title ILIKE $%d", len(args))
	}

	countQuery := "SELECT COUNT(*) FROM market.gigs g WHERE g.deleted_at IS NULL" + whereQ
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		logger.Error("Failed to count gigs", logger.Err(err))
		return nil, 0, err
	}
	if total == 0 {
		return []domain.Gig{}, 0, nil
	}

	args = append(args, limit, (page-1)*limit)
	dataQuery := gigSelect + whereQ +
		fmt.Sprintf(" ORDER BY g.created_at DESC, g.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		logger.Error("Failed to list gigs", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	gigs := make([]domain.Gig, 0)
	for rows.Next() {
		g, err := scanGig(rows)
		if err != nil {
			logger.Error("Failed to scan gig", logger.Err(err))
			return nil, 0, err
		}
		gigs = append(gigs, *g)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate gigs", logger.Err(err))
		return nil, 0, err
	}
	if len(gigs) == 0 {
		return gigs, total, nil
	}

	gigIDs := make([]string, len(gigs))
	for i, g := range gigs {
		gigIDs[i] = g.ID
	}
	tiersByGig, err := r.loadTiers(ctx, gigIDs)
	if err != nil {
		return nil, 0, err
	}
	for i := range gigs {
		gigs[i].Tiers = tiersByGig[gigs[i].ID]
	}

	return gigs, total, nil
}

// GetByID returns one gig with its embedded lapak and price-ascending
// tiers, or ErrGigNotFound for an unknown or soft-deleted id.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Gig, error) {
	query := gigSelect + " AND g.id = $1"

	g, err := scanGig(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGigNotFound
		}
		logger.Error("Failed to get gig", logger.Err(err))
		return nil, err
	}

	tiersByGig, err := r.loadTiers(ctx, []string{g.ID})
	if err != nil {
		return nil, err
	}
	g.Tiers = tiersByGig[g.ID]

	return g, nil
}
