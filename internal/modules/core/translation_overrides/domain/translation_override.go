package domain

import "time"

// TranslationOverride represents an admin-managed i18n override scoped
// to a client (tenant). Two clients can have different values for the
// same translation_key without conflicting.
type TranslationOverride struct {
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
