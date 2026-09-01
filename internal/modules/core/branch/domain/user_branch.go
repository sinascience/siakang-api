package domain

import "time"

// UserBranch represents a user's scoped access to a specific branch.
type UserBranch struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	BranchID  string     `json:"branch_id"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}

// UserBranchDetail joins user_branches with branches so callers get the
// company_id / code / name without a second lookup.
type UserBranchDetail struct {
	BranchID   string `json:"branch_id"`
	BranchCode string `json:"branch_code"`
	BranchName string `json:"branch_name"`
	CompanyID  string `json:"company_id"`
}
