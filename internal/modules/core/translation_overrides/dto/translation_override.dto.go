package dto

import "time"

// CreateTranslationOverrideRequest is the payload for
// POST /admin/clients/:client_id/translation-overrides.
// client_id is taken from the URL, not the body.
type CreateTranslationOverrideRequest struct {
	TranslationKey string  `json:"translation_key" binding:"required,min=1,max=128"`
	Value          string  `json:"value" binding:"required,min=1"`
	Notes          *string `json:"notes" binding:"omitempty"`
}

// UpdateTranslationOverrideRequest is the payload for
// PUT /admin/clients/:client_id/translation-overrides/:key.
type UpdateTranslationOverrideRequest struct {
	Value string  `json:"value" binding:"required,min=1"`
	Notes *string `json:"notes" binding:"omitempty"`
}

// TranslationOverrideResponse is the admin-facing view of a single row.
type TranslationOverrideResponse struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"client_id"`
	TranslationKey string    `json:"translation_key"`
	Value          string    `json:"value"`
	Notes          *string   `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedBy      *string   `json:"created_by,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      *string   `json:"updated_by,omitempty"`
}

// TranslationOverrideListResponse is the admin-facing list payload.
type TranslationOverrideListResponse struct {
	Items []TranslationOverrideResponse `json:"items"`
	Total int64                         `json:"total"`
}

// PublicTranslationsResponse is the flat key→value map consumed by the
// FE at bootstrap time.
type PublicTranslationsResponse struct {
	ClientID     string            `json:"client_id"`
	Slug         string            `json:"slug"`
	Translations map[string]string `json:"translations"`
}
