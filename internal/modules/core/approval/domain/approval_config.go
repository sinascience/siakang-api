package domain

import "time"

// ApprovalConfig is the per (company, branch, feature_key) approval setup.
type ApprovalConfig struct {
	ID         string     `json:"id"`
	CompanyID  string     `json:"company_id"`
	BranchID   string     `json:"branch_id"`
	FeatureKey string     `json:"feature_key"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  *string    `json:"created_by,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
	UpdatedBy  *string    `json:"updated_by,omitempty"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	DeletedBy  *string    `json:"deleted_by,omitempty"`

	Levels []ApprovalConfigLevel `json:"levels,omitempty"`
}

// ApprovalConfigLevel defines one step in the approval chain.
type ApprovalConfigLevel struct {
	ID               string    `json:"id"`
	ApprovalConfigID string    `json:"approval_config_id"`
	Level            int16     `json:"level"`
	RoleID           string    `json:"role_id"`
	ApproverUserIDs  []string  `json:"approver_user_ids"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
