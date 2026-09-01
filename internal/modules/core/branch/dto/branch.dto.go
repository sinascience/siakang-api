package dto

import "time"

// CreateBranchRequest represents request to create a new branch
type CreateBranchRequest struct {
	Code      *string `json:"code" binding:"omitempty,max=50"`
	Name      string  `json:"name" binding:"required,min=2,max=255"`
	LogoURL   *string `json:"logo_url" binding:"omitempty,url,max=500"`
	Sort      *int    `json:"sort" binding:"omitempty,min=0"`
	IsDefault *bool   `json:"is_default"`
}

// UpdateBranchRequest represents request to update a branch
type UpdateBranchRequest struct {
	Code      *string `json:"code" binding:"omitempty,max=50"`
	Name      *string `json:"name" binding:"omitempty,min=2,max=255"`
	LogoURL   *string `json:"logo_url" binding:"omitempty,url,max=500"`
	Sort      *int    `json:"sort" binding:"omitempty,min=0"`
	IsDefault *bool   `json:"is_default"`
	IsActive  *bool   `json:"is_active"`
}

// BranchResponse represents branch data in response
type BranchResponse struct {
	ID        string    `json:"id"`
	CompanyID string    `json:"company_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	LogoURL   *string   `json:"logo_url,omitempty"`
	Sort      int       `json:"sort"`
	IsDefault bool      `json:"is_default"`
	IsActive  bool      `json:"is_active"`
	CreatedBy *string   `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BranchListResponse represents paginated branch list
type BranchListResponse struct {
	Branches []BranchResponse `json:"branches"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
}

// BranchQueryParams represents query parameters for listing branches
type BranchQueryParams struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	Limit    int    `form:"limit,default=10" binding:"min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	IsActive *bool  `form:"is_active"`

	// CompanyIDsRaw is the raw CSV of company UUIDs bound from the query
	// string (e.g. ?company_ids=uuid1,uuid2). Gin does not auto-split CSV
	// into a slice, so the handler splits + validates and fills CompanyIDs.
	CompanyIDsRaw string `form:"company_ids" binding:"omitempty,max=2000"`

	// CompanyIDs is the parsed slice; populated by the handler from
	// CompanyIDsRaw. Not bindable directly.
	CompanyIDs []string `form:"-" json:"-"`

	// ScopeBranchIDs is populated by the handler from the BranchScope
	// middleware; not bindable from the query string.
	ScopeBranchIDs []string `form:"-" json:"-"`
}

// SyncUserBranchesRequest represents a request to sync a user's branch
// access. The branch_ids list is authoritative — entries not present are
// removed (subject to caller scope), entries present are added.
type SyncUserBranchesRequest struct {
	BranchIDs []string `json:"branch_ids" binding:"required,dive,uuid"`
}
