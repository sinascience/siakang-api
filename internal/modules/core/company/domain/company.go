package domain

import "time"

// CompanyType represents the type of company
type CompanyType string

const (
	CompanyTypeHolding    CompanyType = "holding"
	CompanyTypeSubsidiary CompanyType = "subsidiary"
)

// Company represents a company entity with hierarchical structure
type Company struct {
	ID        string      `json:"id"`
	ClientID  string      `json:"client_id"`
	ParentID  *string     `json:"parent_id,omitempty"`
	Name      string      `json:"name"`
	Type      CompanyType `json:"type"`
	LogoURL   *string     `json:"logo_url,omitempty"`
	OwnerID   string      `json:"owner_id"`
	OwnerName *string     `json:"owner_name,omitempty"`
	Sort      int         `json:"sort"`
	IsActive  bool        `json:"is_active"`
	CreatedAt time.Time   `json:"created_at"`
	CreatedBy *string     `json:"created_by,omitempty"`
	UpdatedAt time.Time   `json:"updated_at"`
	UpdatedBy *string     `json:"updated_by,omitempty"`
	DeletedAt *time.Time  `json:"deleted_at,omitempty"`
	DeletedBy *string     `json:"deleted_by,omitempty"`
}

// IsRoot returns true if the company is a root company (no parent)
func (c *Company) IsRoot() bool {
	return c.ParentID == nil
}
