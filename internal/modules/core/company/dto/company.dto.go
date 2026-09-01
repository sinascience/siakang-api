package dto

import "time"

// CreateCompanyRequest represents request to create a new company.
//
// ClientID is only consulted when the caller is super_admin — it lets
// super_admin create a company under a specific client. For regular
// users, ClientID is inferred from the caller's primary company or
// from the parent company (if parent_id is set).
type CreateCompanyRequest struct {
	ClientID *string `json:"client_id" binding:"omitempty,uuid"`
	ParentID *string `json:"parent_id" binding:"omitempty,uuid"`
	OwnerID  *string `json:"owner_id" binding:"omitempty,uuid"`
	Name     string  `json:"name" binding:"required,min=2,max=255"`
	Type     string  `json:"type" binding:"required,oneof=holding subsidiary"`
	LogoURL  *string `json:"logo_url" binding:"omitempty,url,max=500"`
}

// UpdateCompanyRequest represents request to update a company
type UpdateCompanyRequest struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=255"`
	Type     *string `json:"type" binding:"omitempty,oneof=holding subsidiary"`
	OwnerID  *string `json:"owner_id" binding:"omitempty,uuid"`
	LogoURL  *string `json:"logo_url" binding:"omitempty,url,max=500"`
	Sort     *int    `json:"sort" binding:"omitempty,min=0"`
	IsActive *bool   `json:"is_active"`
}

// SyncUserCompaniesRequest represents request to batch sync user-company memberships
type SyncUserCompaniesRequest struct {
	CompanyIDs []string `json:"company_ids" binding:"required"`
	RoleID     *string  `json:"role_id" binding:"omitempty,uuid"`
}

// CompanyResponse represents company data in response
type CompanyResponse struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	LogoURL   *string   `json:"logo_url,omitempty"`
	OwnerID   string    `json:"owner_id"`
	OwnerName *string   `json:"owner_name,omitempty"`
	Sort      int       `json:"sort"`
	IsActive  bool      `json:"is_active"`
	CreatedBy *string   `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CompanyListResponse represents paginated company list (used internally)
type CompanyListResponse struct {
	Companies []CompanyResponse `json:"companies"`
	Total     int64             `json:"total"`
	Page      int               `json:"page"`
	Limit     int               `json:"limit"`
}

// CompanyQueryParams represents query parameters for listing companies
type CompanyQueryParams struct {
	Page     int     `form:"page,default=1" binding:"min=1"`
	Limit    int     `form:"limit,default=10" binding:"min=1,max=100"`
	Search   string  `form:"search" binding:"omitempty,max=255"`
	ParentID *string `form:"parent_id" binding:"omitempty,uuid"`
	Type     *string `form:"type" binding:"omitempty,oneof=holding subsidiary"`
	IsActive *bool   `form:"is_active"`
}

// CompanyTrashQueryParams represents query parameters for listing deleted companies
type CompanyTrashQueryParams struct {
	Page   int    `form:"page,default=1" binding:"min=1"`
	Limit  int    `form:"limit,default=10" binding:"min=1,max=100"`
	Search string `form:"search" binding:"omitempty,max=255"`
}

// AddCompanyUserRequest represents request to add a user to a company
type AddCompanyUserRequest struct {
	UserID string  `json:"user_id" binding:"required,uuid"`
	RoleID *string `json:"role_id" binding:"omitempty,uuid"`
}

// UpdateCompanyUserRequest represents request to update company user membership
type UpdateCompanyUserRequest struct {
	RoleID    *string `json:"role_id" binding:"omitempty,uuid"`
	IsActive  *bool   `json:"is_active"`
	IsPrimary *bool   `json:"is_primary"`
}

// CompanyUserResponse represents company user data in response
type CompanyUserResponse struct {
	ID           string    `json:"id"`
	CompanyID    string    `json:"company_id"`
	UserID       string    `json:"user_id"`
	RoleID       *string   `json:"role_id,omitempty"`
	RoleName     *string   `json:"role_name,omitempty"`
	RoleCode     *string   `json:"role_code,omitempty"`
	UserEmail    *string   `json:"user_email,omitempty"`
	UserUsername *string   `json:"user_username,omitempty"`
	UserFullName *string   `json:"user_full_name,omitempty"`
	IsPrimary    bool      `json:"is_primary"`
	IsActive     bool      `json:"is_active"`
	JoinedAt     time.Time `json:"joined_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// CompanyUserListResponse represents paginated company user list (used internally)
type CompanyUserListResponse struct {
	Users []CompanyUserResponse `json:"users"`
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

// CompanyUserQueryParams represents query parameters for listing company users
type CompanyUserQueryParams struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	Limit    int    `form:"limit,default=10" binding:"min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	IsActive *bool  `form:"is_active"`
}

// UserCompanyIDResponse represents a company ID with ownership flag (for checkbox tree)
type UserCompanyIDResponse struct {
	CompanyID string `json:"company_id"`
	IsOwner   bool   `json:"is_owner"`
}
