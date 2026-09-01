package dto

// MyCompanyResponse represents a company the authenticated user has access to,
// including membership metadata (primary flag, role) and ownership.
type MyCompanyResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	LogoURL   *string `json:"logo_url,omitempty"`
	ParentID  *string `json:"parent_id,omitempty"`
	IsPrimary bool    `json:"is_primary"`
	IsOwner   bool    `json:"is_owner"`
	RoleName  *string `json:"role_name,omitempty"`
	RoleCode  *string `json:"role_code,omitempty"`
}
