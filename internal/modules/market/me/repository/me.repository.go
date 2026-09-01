package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/me/domain"
	"siakang-api/pkg/logger"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindByUserID returns the caller's lapak profile, or (nil, nil) when the
// user is a customer — no market.lapak_profiles row references them. This
// ownership lookup is the entire business logic behind /market/v1/me, so
// there is no service layer forwarding it; the handler calls this directly.
func (r *Repository) FindByUserID(ctx context.Context, userID string) (*domain.LapakProfile, error) {
	const query = `
		SELECT id, user_id, name, description, lat, lng, rating, is_available
		FROM market.lapak_profiles
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	var p domain.LapakProfile
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.Name, &p.Description, &p.Lat, &p.Lng, &p.Rating, &p.IsAvailable,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find lapak profile by user id", logger.Err(err))
		return nil, err
	}
	return &p, nil
}
