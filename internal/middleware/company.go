package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	jwtpkg "siakang-api/pkg/jwt"
)

// CompanyContextVerifier verifies — at request time — that the company in
// the caller's JWT still exists, is active (not soft-deleted), and that the
// caller is still an active member of it. This guards against stale JWTs:
// e.g. a user kicked out of a company would otherwise keep their old
// company_id claim until the token expires.
type CompanyContextVerifier interface {
	VerifyMembership(ctx context.Context, userID, companyID string) error
}

// companyContextVerifier is the global verifier (set by router). nil means
// "verification disabled" (only the JWT-claim presence check runs).
var companyContextVerifier CompanyContextVerifier

// SetCompanyContextVerifier wires the verifier from the router. It's optional
// — if not set, CompanyContext falls back to the legacy behavior of trusting
// whatever is in the JWT.
func SetCompanyContextVerifier(v CompanyContextVerifier) {
	companyContextVerifier = v
}

// CompanyContext middleware ensures that user has a valid company context.
// This middleware should be used after JWTAuth middleware.
//
// When a verifier is configured (see SetCompanyContextVerifier), the company
// + membership are also verified against the database on every request.
// Super admin bypasses the verifier — they are intentionally allowed to
// hold a company_id claim for any tenant.
func CompanyContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get company ID from JWT claims
		companyID := GetCompanyID(c)

		// Check if company ID exists
		if companyID == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Company context is required. User must be associated with a company.",
				"error":   "missing_company_context",
			})
			c.Abort()
			return
		}

		// Live verification against the database (if configured).
		if companyContextVerifier != nil {
			claims, err := GetUserFromContext(c)
			if err == nil && !claims.HasRole(jwtpkg.RoleSuperAdmin) {
				if err := companyContextVerifier.VerifyMembership(c.Request.Context(), claims.UserID, companyID); err != nil {
					c.JSON(http.StatusForbidden, gin.H{
						"success": false,
						"message": "Company context is no longer valid for this user.",
						"error":   "stale_company_context",
					})
					c.Abort()
					return
				}
			}
		}

		// Company ID is valid, continue
		c.Next()
	}
}
