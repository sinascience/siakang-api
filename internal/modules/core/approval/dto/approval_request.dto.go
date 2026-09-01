package dto

import "time"

// ─── REQUEST BODIES ─────────────────────────────────────────────

type ApproveRequest struct {
	Comment string `json:"comment" binding:"omitempty,max=500"`
}

type RejectRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

type CancelRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=500"`
}

// ─── QUERY PARAMS ───────────────────────────────────────────────

type ApprovalRequestQueryParams struct {
	Page       int    `form:"page,default=1" binding:"min=1"`
	Limit      int    `form:"limit,default=20" binding:"min=1,max=100"`
	FeatureKey string `form:"feature_key"`
	BranchID   string `form:"branch_id" binding:"omitempty,uuid"`
	Status     string `form:"status" binding:"omitempty,oneof=waiting approved rejected cancelled"`
	OnlyMine   bool   `form:"only_mine"` // only requests awaiting my action at their current level
}

// ApprovalByDocQueryParams are used by the "by-doc" lookup to fetch the latest
// request for a specific document (reff_type + reff_id), regardless of status.
type ApprovalByDocQueryParams struct {
	ReffType string `form:"reff_type" binding:"required,min=1,max=100"`
	ReffID   string `form:"reff_id" binding:"required,uuid"`
}

// ─── RESPONSES ──────────────────────────────────────────────────

type ApprovalRequestLevelResponse struct {
	ID                  string     `json:"id"`
	Level               int16      `json:"level"`
	RoleID              string     `json:"role_id"`
	RoleName            string     `json:"role_name"`
	EligibleApproverIDs []string   `json:"eligible_approver_ids"`
	Status              string     `json:"status"`
	ActionBy            *string    `json:"action_by,omitempty"`
	ActionAt            *time.Time `json:"action_at,omitempty"`
	Comment             *string    `json:"comment,omitempty"`
}

type ApprovalRequestResponse struct {
	ID           string                         `json:"id"`
	CompanyID    string                         `json:"company_id"`
	BranchID     string                         `json:"branch_id"`
	FeatureKey   string                         `json:"feature_key"`
	ReffType     string                         `json:"reff_type"`
	ReffID       string                         `json:"reff_id"`
	Status       string                         `json:"status"`
	CurrentLevel *int16                         `json:"current_level,omitempty"`
	TotalLevels  int16                          `json:"total_levels"`
	SubmittedAt  time.Time                      `json:"submitted_at"`
	SubmittedBy  string                         `json:"submitted_by"`
	FinalizedAt  *time.Time                     `json:"finalized_at,omitempty"`
	FinalizedBy  *string                        `json:"finalized_by,omitempty"`
	RejectReason *string                        `json:"reject_reason,omitempty"`
	Levels       []ApprovalRequestLevelResponse `json:"levels,omitempty"`
}

// ─── INTERNAL (used by other modules via service call) ─────────

// SubmitParams is what feature modules pass when submitting a document for
// approval. Not a JSON DTO — used across package boundaries.
type SubmitParams struct {
	CompanyID   string
	BranchID    string
	FeatureKey  string
	ReffType    string
	ReffID      string
	SubmittedBy string
}

// SubmitResult is returned from Submit. When AutoApproved is true, the request
// is already in 'approved' status and total_levels == 0.
type SubmitResult struct {
	Request      *ApprovalRequestResponse
	AutoApproved bool
}
