package dto

// SwitchCompanyRequest represents the switch company request payload
type SwitchCompanyRequest struct {
	CompanyID string `json:"company_id" binding:"required,uuid"`
}

// SwitchCompanyResponse represents the switch company response.
//
// Roles and Permissions are returned here because they differ per
// company — the FE can no longer decode them from the JWT (permissions
// are stored server-side in Redis rather than embedded in the token).
type SwitchCompanyResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	Company      CompanyInfo `json:"company"`
	Roles        []string    `json:"roles"`
	Permissions  []string    `json:"permissions"`
}
