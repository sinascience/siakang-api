package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/core/client/domain"
	"siakang-api/pkg/logger"
)

var ErrNotFound = errors.New("client not found")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Pool returns the underlying connection pool. Exposed so callers that
// need to reuse the pool as an Execer (non-transactional code paths)
// can do so without re-plumbing a dependency.
func (r *Repository) Pool() *pgxpool.Pool {
	return r.db
}

// Execer abstracts over *pgxpool.Pool and pgx.Tx so callers can run the
// same query inside an outer transaction (used by auth SignUp).
type Execer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// CreateTx inserts a client using the caller-provided execer. Uses
// RETURNING so the populated timestamps flow back into the domain.
func (r *Repository) CreateTx(ctx context.Context, q Execer, c *domain.Client) error {
	query := `
		INSERT INTO core.clients (
			id, slug, name, owner_user_id, is_active,
			created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING created_at, updated_at
	`
	err := q.QueryRow(ctx, query,
		c.ID, c.Slug, c.Name, c.OwnerUserID, c.IsActive,
		c.CreatedBy,
	).Scan(&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		logger.Error("Failed to create client", logger.Err(err))
		return err
	}
	c.UpdatedBy = c.CreatedBy
	return nil
}

// Create is the non-transactional shortcut (used by admin-authored code paths).
func (r *Repository) Create(ctx context.Context, c *domain.Client) error {
	return r.CreateTx(ctx, r.db, c)
}

// FindByID returns the client with the given id (non-deleted only).
func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Client, error) {
	return r.findOne(ctx,
		`WHERE id = $1 AND deleted_at IS NULL`, id)
}

// FindBySlug resolves a slug to a client. Used by the public
// translation-bootstrap endpoint — callers supply ErrNotFound as 404.
func (r *Repository) FindBySlug(ctx context.Context, slug string) (*domain.Client, error) {
	return r.findOne(ctx,
		`WHERE slug = $1 AND deleted_at IS NULL`, slug)
}

// FindByOwnerUserID returns the (first) client owned by the user, if any.
// Callers treat the nil, nil return as "user has not registered a client yet".
func (r *Repository) FindByOwnerUserID(ctx context.Context, ownerID string) (*domain.Client, error) {
	c, err := r.findOne(ctx,
		`WHERE owner_user_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at ASC
		 LIMIT 1`, ownerID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return c, err
}

func (r *Repository) findOne(ctx context.Context, whereClause string, args ...any) (*domain.Client, error) {
	query := `
		SELECT id, slug, name, owner_user_id, is_active,
		       created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		FROM core.clients
	` + whereClause
	var c domain.Client
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.Slug, &c.Name, &c.OwnerUserID, &c.IsActive,
		&c.CreatedAt, &c.CreatedBy, &c.UpdatedAt, &c.UpdatedBy,
		&c.DeletedAt, &c.DeletedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		logger.Error("Failed to scan client", logger.Err(err))
		return nil, err
	}
	return &c, nil
}

// FindAll returns every non-deleted client sorted by created_at DESC.
func (r *Repository) FindAll(ctx context.Context) ([]domain.Client, error) {
	query := `
		SELECT id, slug, name, owner_user_id, is_active,
		       created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		FROM core.clients
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		logger.Error("Failed to list clients", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var out []domain.Client
	for rows.Next() {
		var c domain.Client
		if err := rows.Scan(
			&c.ID, &c.Slug, &c.Name, &c.OwnerUserID, &c.IsActive,
			&c.CreatedAt, &c.CreatedBy, &c.UpdatedAt, &c.UpdatedBy,
			&c.DeletedAt, &c.DeletedBy,
		); err != nil {
			logger.Error("Failed to scan client row", logger.Err(err))
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SlugExists returns true if the slug is already taken by a non-deleted
// row. Used during signup to detect collisions before insert so we can
// append a numeric suffix instead of crashing on the unique constraint.
func (r *Repository) SlugExists(ctx context.Context, q Execer, slug string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM core.clients
			WHERE slug = $1 AND deleted_at IS NULL
		)`, slug,
	).Scan(&exists)
	return exists, err
}

// Update mutates name/slug/is_active. Returns ErrNotFound if missing.
func (r *Repository) Update(ctx context.Context, id string, name, slug *string, isActive *bool, updatedBy string) (*domain.Client, error) {
	query := `
		UPDATE core.clients
		SET name      = COALESCE($2, name),
		    slug      = COALESCE($3, slug),
		    is_active = COALESCE($4, is_active),
		    updated_at = NOW(),
		    updated_by = $5
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, slug, name, owner_user_id, is_active,
		          created_at, created_by, updated_at, updated_by,
		          deleted_at, deleted_by
	`
	var c domain.Client
	err := r.db.QueryRow(ctx, query, id, name, slug, isActive, updatedBy).Scan(
		&c.ID, &c.Slug, &c.Name, &c.OwnerUserID, &c.IsActive,
		&c.CreatedAt, &c.CreatedBy, &c.UpdatedAt, &c.UpdatedBy,
		&c.DeletedAt, &c.DeletedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		logger.Error("Failed to update client", logger.Err(err))
		return nil, err
	}
	return &c, nil
}
