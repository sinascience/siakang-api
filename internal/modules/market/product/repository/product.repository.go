package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/product/domain"
	"siakang-api/pkg/logger"
)

// ErrProductNotFound means no non-deleted market.products row matches the id.
var ErrProductNotFound = errors.New("product not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// productSelect joins each product to its selling lapak so both list and
// detail return the embedded LapakSummary in one query, never a per-row
// lookup in a loop.
const productSelect = `
	SELECT p.id, p.title, p.description, p.price_idr, p.image_url,
	       l.id, l.name, l.rating
	FROM market.products p
	JOIN market.lapak_profiles l ON l.id = p.lapak_id AND l.deleted_at IS NULL
	WHERE p.deleted_at IS NULL
`

func scanProduct(row pgx.Row) (*domain.Product, error) {
	var p domain.Product
	err := row.Scan(
		&p.ID, &p.Title, &p.Description, &p.PriceIDR, &p.ImageURL,
		&p.LapakID, &p.LapakName, &p.LapakRating,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns the catalog page, newest first, optionally filtered by a
// case-insensitive substring match on title. total reflects the FILTERED
// set (a q that matches two rows reports total 2), so it is counted with
// the same WHERE clause the page query uses.
//
// ponytail: q is a plain ILIKE, a sequential scan at seed scale — already
// flagged in the products migration; add pg_trgm + a GIN index if the
// catalog grows enough for it to show up in a query plan.
func (r *Repository) List(ctx context.Context, page, limit int, q string) ([]domain.Product, int64, error) {
	var args []interface{}
	whereQ := ""
	if q != "" {
		args = append(args, "%"+q+"%")
		whereQ = fmt.Sprintf(" AND p.title ILIKE $%d", len(args))
	}

	countQuery := "SELECT COUNT(*) FROM market.products p WHERE p.deleted_at IS NULL" + whereQ
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		logger.Error("Failed to count products", logger.Err(err))
		return nil, 0, err
	}
	if total == 0 {
		return []domain.Product{}, 0, nil
	}

	args = append(args, limit, (page-1)*limit)
	dataQuery := productSelect + whereQ +
		fmt.Sprintf(" ORDER BY p.created_at DESC, p.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		logger.Error("Failed to list products", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	products := make([]domain.Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			logger.Error("Failed to scan product", logger.Err(err))
			return nil, 0, err
		}
		products = append(products, *p)
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to iterate products", logger.Err(err))
		return nil, 0, err
	}

	return products, total, nil
}

// GetByID returns one product with its embedded lapak, or ErrProductNotFound
// for an unknown or soft-deleted id.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	query := productSelect + " AND p.id = $1"

	p, err := scanProduct(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		logger.Error("Failed to get product", logger.Err(err))
		return nil, err
	}
	return p, nil
}
