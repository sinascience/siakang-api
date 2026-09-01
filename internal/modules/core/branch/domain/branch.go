package domain

import "time"

// Branch represents a branch office belonging to a company
type Branch struct {
	ID        string     `json:"id"`
	CompanyID string     `json:"company_id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	LogoURL   *string    `json:"logo_url,omitempty"`
	Sort      int        `json:"sort"`
	IsDefault bool       `json:"is_default"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}
