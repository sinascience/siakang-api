package domain

import "time"

// CompanyUser represents a user's membership in a company
type CompanyUser struct {
	ID        string     `json:"id"`
	CompanyID string     `json:"company_id"`
	UserID    string     `json:"user_id"`
	RoleID    *string    `json:"role_id,omitempty"`
	IsPrimary bool       `json:"is_primary"`
	IsActive  bool       `json:"is_active"`
	InvitedBy *string    `json:"invited_by,omitempty"`
	JoinedAt  time.Time  `json:"joined_at"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}

// UserCompanyDetail represents a user's company membership with company and role info
type UserCompanyDetail struct {
	CompanyID   string  `json:"company_id"`
	CompanyName string  `json:"company_name"`
	CompanyType string  `json:"company_type"`
	LogoURL     *string `json:"logo_url,omitempty"`
	ParentID    *string `json:"parent_id,omitempty"`
	OwnerID     string  `json:"owner_id"`
	IsPrimary   bool    `json:"is_primary"`
	RoleName    *string `json:"role_name,omitempty"`
	RoleCode    *string `json:"role_code,omitempty"`
}

// CompanyUserWithDetails represents company user with user details
type CompanyUserWithDetails struct {
	CompanyUser
	UserEmail    *string `json:"user_email,omitempty"`
	UserUsername *string `json:"user_username,omitempty"`
	UserFullName *string `json:"user_full_name,omitempty"`
	RoleName     *string `json:"role_name,omitempty"`
	RoleCode     *string `json:"role_code,omitempty"`
}
