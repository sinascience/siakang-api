package domain

import "time"

// Request lifecycle statuses.
const (
	RequestStatusWaiting   = "waiting"
	RequestStatusApproved  = "approved"
	RequestStatusRejected  = "rejected"
	RequestStatusCancelled = "cancelled"
)

// Level action statuses.
const (
	LevelStatusPending  = "pending"
	LevelStatusApproved = "approved"
	LevelStatusRejected = "rejected"
	LevelStatusSkipped  = "skipped"
)

// ApprovalRequest is the runtime instance of an approval chain for a single
// target document. Target is referenced polymorphically via reff_type/reff_id.
type ApprovalRequest struct {
	ID               string     `json:"id"`
	CompanyID        string     `json:"company_id"`
	BranchID         string     `json:"branch_id"`
	FeatureKey       string     `json:"feature_key"`
	ReffType         string     `json:"reff_type"`
	ReffID           string     `json:"reff_id"`
	ApprovalConfigID *string    `json:"approval_config_id,omitempty"`
	Status           string     `json:"status"`
	CurrentLevel     *int16     `json:"current_level,omitempty"`
	TotalLevels      int16      `json:"total_levels"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	SubmittedBy      string     `json:"submitted_by"`
	FinalizedAt      *time.Time `json:"finalized_at,omitempty"`
	FinalizedBy      *string    `json:"finalized_by,omitempty"`
	RejectReason     *string    `json:"reject_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`

	Levels []ApprovalRequestLevel `json:"levels,omitempty"`
}

// ApprovalRequestLevel is a per-level audit row. Role & approver IDs are
// snapshotted at submission time so config changes don't rewrite history.
type ApprovalRequestLevel struct {
	ID                  string     `json:"id"`
	ApprovalRequestID   string     `json:"approval_request_id"`
	Level               int16      `json:"level"`
	RoleID              string     `json:"role_id"`
	RoleName            string     `json:"role_name"`
	EligibleApproverIDs []string   `json:"eligible_approver_ids"`
	Status              string     `json:"status"`
	ActionBy            *string    `json:"action_by,omitempty"`
	ActionAt            *time.Time `json:"action_at,omitempty"`
	Comment             *string    `json:"comment,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}
