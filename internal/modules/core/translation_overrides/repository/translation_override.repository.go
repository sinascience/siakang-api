package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/core/translation_overrides/domain"
	"siakang-api/pkg/logger"
)

var ErrNotFound = errors.New("translation override not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new override for the given client.
func (r *Repository) Create(ctx context.Context, to *domain.TranslationOverride) error {
	query := `
		INSERT INTO core.translation_overrides (
			client_id, translation_key, value, notes, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		to.ClientID, to.TranslationKey, to.Value, to.Notes, to.CreatedBy,
	).Scan(&to.ID, &to.CreatedAt, &to.UpdatedAt)
	if err != nil {
		logger.Error("Failed to create translation override", logger.Err(err))
		return err
	}
	to.UpdatedBy = to.CreatedBy
	return nil
}

// FindByClientKey returns the override for (client_id, translation_key).
func (r *Repository) FindByClientKey(ctx context.Context, clientID, key string) (*domain.TranslationOverride, error) {
	query := `
		SELECT id, client_id, translation_key, value, notes,
		       created_at, created_by, updated_at, updated_by
		FROM core.translation_overrides
		WHERE client_id = $1 AND translation_key = $2
	`
	var to domain.TranslationOverride
	err := r.db.QueryRow(ctx, query, clientID, key).Scan(
		&to.ID, &to.ClientID, &to.TranslationKey, &to.Value, &to.Notes,
		&to.CreatedAt, &to.CreatedBy, &to.UpdatedAt, &to.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		logger.Error("Failed to fetch translation override", logger.Err(err))
		return nil, err
	}
	return &to, nil
}

// FindAllForClient returns every override owned by the given client.
func (r *Repository) FindAllForClient(ctx context.Context, clientID string) ([]domain.TranslationOverride, error) {
	query := `
		SELECT id, client_id, translation_key, value, notes,
		       created_at, created_by, updated_at, updated_by
		FROM core.translation_overrides
		WHERE client_id = $1
		ORDER BY updated_at DESC
	`
	rows, err := r.db.Query(ctx, query, clientID)
	if err != nil {
		logger.Error("Failed to list translation overrides", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var out []domain.TranslationOverride
	for rows.Next() {
		var to domain.TranslationOverride
		if err := rows.Scan(
			&to.ID, &to.ClientID, &to.TranslationKey, &to.Value, &to.Notes,
			&to.CreatedAt, &to.CreatedBy, &to.UpdatedAt, &to.UpdatedBy,
		); err != nil {
			logger.Error("Failed to scan translation override row", logger.Err(err))
			return nil, err
		}
		out = append(out, to)
	}
	return out, rows.Err()
}

// UpdateByClientKey updates value/notes for an existing override.
func (r *Repository) UpdateByClientKey(
	ctx context.Context,
	clientID, key, value string,
	notes *string,
	updatedBy *string,
) (*domain.TranslationOverride, error) {
	query := `
		UPDATE core.translation_overrides
		SET value = $3, notes = $4, updated_at = NOW(), updated_by = $5
		WHERE client_id = $1 AND translation_key = $2
		RETURNING id, client_id, translation_key, value, notes,
		          created_at, created_by, updated_at, updated_by
	`
	var to domain.TranslationOverride
	err := r.db.QueryRow(ctx, query, clientID, key, value, notes, updatedBy).Scan(
		&to.ID, &to.ClientID, &to.TranslationKey, &to.Value, &to.Notes,
		&to.CreatedAt, &to.CreatedBy, &to.UpdatedAt, &to.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		logger.Error("Failed to update translation override", logger.Err(err))
		return nil, err
	}
	return &to, nil
}

// DeleteByClientKey removes an override for the given client/key.
func (r *Repository) DeleteByClientKey(ctx context.Context, clientID, key string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM core.translation_overrides
		 WHERE client_id = $1 AND translation_key = $2`,
		clientID, key,
	)
	if err != nil {
		logger.Error("Failed to delete translation override", logger.Err(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
