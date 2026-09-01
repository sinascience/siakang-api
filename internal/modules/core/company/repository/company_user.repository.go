package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/core/company/domain"
	"siakang-api/internal/modules/core/company/dto"
	"siakang-api/pkg/logger"
)

type CompanyUserRepository struct {
	db *pgxpool.Pool
}

func NewCompanyUserRepository(db *pgxpool.Pool) *CompanyUserRepository {
	return &CompanyUserRepository{db: db}
}

// Create creates a new company user membership
func (r *CompanyUserRepository) Create(ctx context.Context, cu *domain.CompanyUser) error {
	return r.createWith(ctx, r.db, cu)
}

// CreateTx creates a new company user membership inside an existing
// transaction. Use this together with UserRepository.CreateTx so that user
// creation and membership assignment are atomic.
func (r *CompanyUserRepository) CreateTx(ctx context.Context, tx pgx.Tx, cu *domain.CompanyUser) error {
	return r.createWith(ctx, tx, cu)
}

// dbExec is the minimal subset of *pgxpool.Pool / pgx.Tx that we need for
// shared write helpers. Both types satisfy this implicitly.
type dbExec interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *CompanyUserRepository) createWith(ctx context.Context, q dbExec, cu *domain.CompanyUser) error {
	query := `
		INSERT INTO core.company_users (
			id, company_id, user_id, role_id, is_primary, is_active,
			invited_by, joined_at, created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := q.Exec(ctx, query,
		cu.ID, cu.CompanyID, cu.UserID, cu.RoleID, cu.IsPrimary, cu.IsActive,
		cu.InvitedBy, cu.JoinedAt, cu.CreatedAt, cu.CreatedBy, cu.UpdatedAt, cu.UpdatedBy,
	)

	if err != nil {
		logger.Error("Failed to create company user", logger.Err(err))
		return err
	}

	return nil
}

// FindByID finds a company user by ID
func (r *CompanyUserRepository) FindByID(ctx context.Context, id string) (*domain.CompanyUser, error) {
	query := `
		SELECT id, company_id, user_id, role_id, is_primary, is_active,
		       invited_by, joined_at, created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		FROM core.company_users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var cu domain.CompanyUser
	err := r.db.QueryRow(ctx, query, id).Scan(
		&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.IsPrimary, &cu.IsActive,
		&cu.InvitedBy, &cu.JoinedAt, &cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
		&cu.DeletedAt, &cu.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find company user by ID", logger.Err(err))
		return nil, err
	}

	return &cu, nil
}

// FindByCompanyAndUser finds a company user by company ID and user ID
func (r *CompanyUserRepository) FindByCompanyAndUser(ctx context.Context, companyID, userID string) (*domain.CompanyUser, error) {
	query := `
		SELECT id, company_id, user_id, role_id, is_primary, is_active,
		       invited_by, joined_at, created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		FROM core.company_users
		WHERE company_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var cu domain.CompanyUser
	err := r.db.QueryRow(ctx, query, companyID, userID).Scan(
		&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.IsPrimary, &cu.IsActive,
		&cu.InvitedBy, &cu.JoinedAt, &cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
		&cu.DeletedAt, &cu.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to find company user", logger.Err(err))
		return nil, err
	}

	return &cu, nil
}

// VerifyMembership returns nil if the user has an active, non-deleted
// membership in the given company AND the company itself still exists and
// is active (not soft-deleted, is_active = true). Returns an error otherwise.
//
// Used by middleware.CompanyContext to invalidate stale JWT company claims
// (user kicked out, company soft-deleted, company deactivated).
func (r *CompanyUserRepository) VerifyMembership(ctx context.Context, userID, companyID string) error {
	const query = `
		SELECT 1
		FROM core.company_users cu
		JOIN core.companies c ON c.id = cu.company_id
		WHERE cu.user_id = $1
		  AND cu.company_id = $2
		  AND cu.is_active = true
		  AND cu.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		  AND c.is_active = true
		LIMIT 1
	`

	var one int
	err := r.db.QueryRow(ctx, query, userID, companyID).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("stale company context")
		}
		logger.Error("Failed to verify company membership", logger.Err(err))
		return err
	}
	return nil
}

// IsMember checks if a user is a member of a company
func (r *CompanyUserRepository) IsMember(ctx context.Context, userID, companyID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM core.company_users
			WHERE user_id = $1 AND company_id = $2 AND is_active = true AND deleted_at IS NULL
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, userID, companyID).Scan(&exists)
	if err != nil {
		logger.Error("Failed to check membership", logger.Err(err))
		return false, err
	}

	return exists, nil
}

// FindByCompany finds all users in a company with pagination
func (r *CompanyUserRepository) FindByCompany(ctx context.Context, companyID string, params *dto.CompanyUserQueryParams) ([]domain.CompanyUserWithDetails, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	conditions = append(conditions, fmt.Sprintf("cu.company_id = $%d", argIndex))
	args = append(args, companyID)
	argIndex++

	conditions = append(conditions, "cu.deleted_at IS NULL")

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(u.email ILIKE $%d OR u.username ILIKE $%d OR u.full_name ILIKE $%d)",
			argIndex, argIndex, argIndex,
		))
		args = append(args, "%"+params.Search+"%")
		argIndex++
	}

	if params.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("cu.is_active = $%d", argIndex))
		args = append(args, *params.IsActive)
		argIndex++
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM core.company_users cu
		JOIN core.users u ON u.id = cu.user_id
		WHERE %s
	`, whereClause)
	var total int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		logger.Error("Failed to count company users", logger.Err(err))
		return nil, 0, err
	}

	// Data query
	offset := (params.Page - 1) * params.Limit
	dataQuery := fmt.Sprintf(`
		SELECT cu.id, cu.company_id, cu.user_id, cu.role_id, cu.is_primary, cu.is_active,
		       cu.invited_by, cu.joined_at, cu.created_at, cu.created_by, cu.updated_at, cu.updated_by,
		       cu.deleted_at, cu.deleted_by,
		       u.email, u.username, u.full_name,
		       r.name, r.code
		FROM core.company_users cu
		JOIN core.users u ON u.id = cu.user_id
		LEFT JOIN core.roles r ON r.id = cu.role_id
		WHERE %s
		ORDER BY cu.is_primary DESC, u.full_name ASC, u.username ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, params.Limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		logger.Error("Failed to find company users", logger.Err(err))
		return nil, 0, err
	}
	defer rows.Close()

	var users []domain.CompanyUserWithDetails
	for rows.Next() {
		var cu domain.CompanyUserWithDetails
		err := rows.Scan(
			&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.IsPrimary, &cu.IsActive,
			&cu.InvitedBy, &cu.JoinedAt, &cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
			&cu.DeletedAt, &cu.DeletedBy,
			&cu.UserEmail, &cu.UserUsername, &cu.UserFullName,
			&cu.RoleName, &cu.RoleCode,
		)
		if err != nil {
			logger.Error("Failed to scan company user", logger.Err(err))
			return nil, 0, err
		}
		users = append(users, cu)
	}

	return users, total, nil
}

// FindByUser finds all companies for a user
func (r *CompanyUserRepository) FindByUser(ctx context.Context, userID string) ([]domain.CompanyUser, error) {
	query := `
		SELECT id, company_id, user_id, role_id, is_primary, is_active,
		       invited_by, joined_at, created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		FROM core.company_users
		WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL
		ORDER BY is_primary DESC, joined_at ASC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.Error("Failed to find companies by user", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var memberships []domain.CompanyUser
	for rows.Next() {
		var cu domain.CompanyUser
		err := rows.Scan(
			&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.IsPrimary, &cu.IsActive,
			&cu.InvitedBy, &cu.JoinedAt, &cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
			&cu.DeletedAt, &cu.DeletedBy,
		)
		if err != nil {
			logger.Error("Failed to scan company user", logger.Err(err))
			return nil, err
		}
		memberships = append(memberships, cu)
	}

	return memberships, nil
}

// FindCompanyIDsByUser returns only the company_ids a user is an active member of.
// Lightweight variant of FindByUser for callers that just need the scope list.
func (r *CompanyUserRepository) FindCompanyIDsByUser(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT company_id
		FROM core.company_users
		WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.Error("Failed to find company ids by user", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			logger.Error("Failed to scan company id", logger.Err(err))
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Update updates a company user membership
func (r *CompanyUserRepository) Update(ctx context.Context, cu *domain.CompanyUser) error {
	query := `
		UPDATE core.company_users
		SET role_id = $2, is_primary = $3, is_active = $4,
		    updated_at = $5, updated_by = $6
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query,
		cu.ID, cu.RoleID, cu.IsPrimary, cu.IsActive,
		cu.UpdatedAt, cu.UpdatedBy,
	)

	if err != nil {
		logger.Error("Failed to update company user", logger.Err(err))
		return err
	}

	return nil
}

// SoftDelete soft deletes a company user membership
func (r *CompanyUserRepository) SoftDelete(ctx context.Context, id, deletedBy string) error {
	query := `
		UPDATE core.company_users
		SET deleted_at = $2, deleted_by = $3, updated_at = $2, updated_by = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	now := time.Now()
	_, err := r.db.Exec(ctx, query, id, now, deletedBy)
	if err != nil {
		logger.Error("Failed to soft delete company user", logger.Err(err))
		return err
	}

	return nil
}

// SetPrimaryCompany sets a company as the user's primary company
func (r *CompanyUserRepository) SetPrimaryCompany(ctx context.Context, userID, companyID string) error {
	// First, unset current primary
	unsetQuery := `
		UPDATE core.company_users
		SET is_primary = false, updated_at = $2
		WHERE user_id = $1 AND is_primary = true AND deleted_at IS NULL
	`
	_, err := r.db.Exec(ctx, unsetQuery, userID, time.Now())
	if err != nil {
		logger.Error("Failed to unset primary company", logger.Err(err))
		return err
	}

	// Then, set new primary
	setQuery := `
		UPDATE core.company_users
		SET is_primary = true, updated_at = $3
		WHERE user_id = $1 AND company_id = $2 AND deleted_at IS NULL
	`
	_, err = r.db.Exec(ctx, setQuery, userID, companyID, time.Now())
	if err != nil {
		logger.Error("Failed to set primary company", logger.Err(err))
		return err
	}

	return nil
}

// GetPrimaryCompany gets the user's primary company
func (r *CompanyUserRepository) GetPrimaryCompany(ctx context.Context, userID string) (*domain.CompanyUser, error) {
	query := `
		SELECT id, company_id, user_id, role_id, is_primary, is_active,
		       invited_by, joined_at, created_at, created_by, updated_at, updated_by,
		       deleted_at, deleted_by
		FROM core.company_users
		WHERE user_id = $1 AND is_primary = true AND is_active = true AND deleted_at IS NULL
	`

	var cu domain.CompanyUser
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.IsPrimary, &cu.IsActive,
		&cu.InvitedBy, &cu.JoinedAt, &cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
		&cu.DeletedAt, &cu.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error("Failed to get primary company", logger.Err(err))
		return nil, err
	}

	return &cu, nil
}

// FindByUserWithDetails finds all companies for a user with company and role info
func (r *CompanyUserRepository) FindByUserWithDetails(ctx context.Context, userID string) ([]domain.UserCompanyDetail, error) {
	query := `
		SELECT cu.company_id, c.name, c.type, c.logo_url, c.parent_id, c.owner_id,
		       cu.is_primary, r.name, r.code
		FROM core.company_users cu
		JOIN core.companies c ON c.id = cu.company_id AND c.deleted_at IS NULL
		LEFT JOIN core.roles r ON r.id = cu.role_id AND r.deleted_at IS NULL
		WHERE cu.user_id = $1 AND cu.is_active = true AND cu.deleted_at IS NULL
		ORDER BY cu.is_primary DESC, c.name ASC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.Error("Failed to find user companies with details", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	var results []domain.UserCompanyDetail
	for rows.Next() {
		var d domain.UserCompanyDetail
		err := rows.Scan(
			&d.CompanyID, &d.CompanyName, &d.CompanyType, &d.LogoURL, &d.ParentID, &d.OwnerID,
			&d.IsPrimary, &d.RoleName, &d.RoleCode,
		)
		if err != nil {
			logger.Error("Failed to scan user company detail", logger.Err(err))
			return nil, err
		}
		results = append(results, d)
	}

	return results, nil
}

// MembershipExists checks if a membership already exists
func (r *CompanyUserRepository) MembershipExists(ctx context.Context, companyID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM core.company_users
			WHERE company_id = $1 AND user_id = $2 AND deleted_at IS NULL
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, companyID, userID).Scan(&exists)
	if err != nil {
		logger.Error("Failed to check membership existence", logger.Err(err))
		return false, err
	}

	return exists, nil
}
