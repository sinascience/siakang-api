package dto

import "time"

// ClientResponse is the admin-facing view of a client.
type ClientResponse struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	OwnerUserID *string   `json:"owner_user_id,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ClientListResponse wraps a non-paginated list (clients are low-volume).
type ClientListResponse struct {
	Items []ClientResponse `json:"items"`
	Total int64            `json:"total"`
}

// UpdateClientRequest is the admin payload for renaming / re-slugging.
type UpdateClientRequest struct {
	Name     *string `json:"name" binding:"omitempty,min=1,max=255"`
	Slug     *string `json:"slug" binding:"omitempty,min=3,max=63"`
	IsActive *bool   `json:"is_active" binding:"omitempty"`
}
