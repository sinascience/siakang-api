package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/core/branch/domain"
	"siakang-api/pkg/logger"
)

type UserBranchRepository struct {
	db *pgxpool.Pool
}

func NewUserBranchRepository(db *pgxpool.Pool) *UserBranchRepository {
	return &UserBranchRepository{db: db}
}

// userBranchExec is the minimal subset of *pgxpool.Pool / pgx.Tx we need.
type userBranchExec interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Create inserts a new user_branches row.
func (r *UserBranchRepository) Create(ctx context.Context, ub *domain.UserBranch) error {
	return r.createWith(ctx, r.db, ub)
}

// CreateTx inserts a new user_branches row inside an existing transaction.
// Used by the user create flow so user + memberships + branch access commit
// atomically.
func (r *UserBranchRepository) CreateTx(ctx context.Context, tx pgx.Tx, ub *domain.UserBranch) error {
	return r.createWith(ctx, tx, ub)
}

func (r *UserBranchRepository) createWith(ctx context.Context, q userBranchExec, ub *domain.UserBranch) error {
	const query = `
		INSERT INTO core.user_branches (
			id, user_id, branch_id,
			created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := q.Exec(ctx, query,
		ub.ID, ub.UserID, ub.BranchID,
		ub.CreatedAt, ub.CreatedBy, ub.UpdatedAt, ub.UpdatedBy,
	)
	if err != nil {
		logger.Error("Failed to create user_branch", logger.Err(err))
		return err
	}
	return nil
}

// FindBranchIDsByUser returns just the active branch IDs a user has access
// to — hot path used by the BranchScope middleware on every scoped request.
// Keeps the query minimal to avoid the per-request overhead of FindByUser,
// which returns full rows.
func (r *UserBranchRepository) FindBranchIDsByUser(ctx context.Context, userID string) ([]string, error) {
	const query = `
		SELECT branch_id
		FROM core.user_branches
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.Error("Failed to find user branch ids", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			logger.Error("Failed to scan user branch id", logger.Err(err))
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// FindByUser returns the active branch IDs a user has access to.
func (r *UserBranchRepository) FindByUser(ctx context.Context, userID string) ([]domain.UserBranch, error) {
	const query = `
		SELECT id, user_id, branch_id,
		       created_at, created_by, updated_at, updated_by, deleted_at, deleted_by
		FROM core.user_branches
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.Error("Failed to find user branches", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var out []domain.UserBranch
	for rows.Next() {
		var ub domain.UserBranch
		if err := rows.Scan(
			&ub.ID, &ub.UserID, &ub.BranchID,
			&ub.CreatedAt, &ub.CreatedBy, &ub.UpdatedAt, &ub.UpdatedBy,
			&ub.DeletedAt, &ub.DeletedBy,
		); err != nil {
			logger.Error("Failed to scan user_branch", logger.Err(err))
			return nil, err
		}
		out = append(out, ub)
	}
	return out, nil
}

// FindByUserWithDetails returns branch details (code/name/company_id) for a
// user. Only active (non-deleted, non-deleted branch) rows are returned.
func (r *UserBranchRepository) FindByUserWithDetails(ctx context.Context, userID string) ([]domain.UserBranchDetail, error) {
	const query = `
		SELECT b.id, b.code, b.name, b.company_id
		FROM core.user_branches ub
		JOIN core.branches b ON b.id = ub.branch_id AND b.deleted_at IS NULL
		WHERE ub.user_id = $1 AND ub.deleted_at IS NULL
		ORDER BY b.company_id, b.sort, b.name
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.Error("Failed to find user branch details", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var out []domain.UserBranchDetail
	for rows.Next() {
		var d domain.UserBranchDetail
		if err := rows.Scan(&d.BranchID, &d.BranchCode, &d.BranchName, &d.CompanyID); err != nil {
			logger.Error("Failed to scan user branch detail", logger.Err(err))
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// SoftDelete marks a user_branches row as deleted.
func (r *UserBranchRepository) SoftDelete(ctx context.Context, id, deletedBy string) error {
	const query = `
		UPDATE core.user_branches
		SET deleted_at = $2, deleted_by = $3, updated_at = $2, updated_by = $3
		WHERE id = $1 AND deleted_at IS NULL
	`
	now := time.Now()
	if _, err := r.db.Exec(ctx, query, id, now, deletedBy); err != nil {
		logger.Error("Failed to soft delete user_branch", logger.Err(err))
		return err
	}
	return nil
}

// FindBranchesByIDsWithCompany returns the branches that match the given IDs
// (skipping soft-deleted ones) along with their company_id. The caller uses
// this to validate that every requested branch exists AND that it belongs to
// a company the caller is allowed to assign under.
func (r *UserBranchRepository) FindBranchesByIDsWithCompany(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	const query = `
		SELECT id, company_id
		FROM core.branches
		WHERE id = ANY($1) AND deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		logger.Error("Failed to resolve branches by ids", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string, len(ids))
	for rows.Next() {
		var bid, cid string
		if err := rows.Scan(&bid, &cid); err != nil {
			logger.Error("Failed to scan branch id/company_id", logger.Err(err))
			return nil, err
		}
		out[bid] = cid
	}
	return out, nil
}
