package middleware

import (
	"net/http"

	"siakang-api/internal/shared/response"
	"siakang-api/pkg/logger"

	"github.com/gin-gonic/gin"
)

// RequireClientScope enforces that the caller's JWT client_id matches
// the URL :id param (the client being acted on). super_admin bypasses.
// Must be used AFTER JWTAuth.
//
// Use this to guard per-client admin endpoints so client A cannot touch
// client B's data — while still letting client A's users manage their
// OWN per-client resources without needing the super_admin role.
//
// Rejects with 403 if:
//   - JWT has no client_id (token issued before the client-migration)
//   - JWT's client_id doesn't match :id
func RequireClientScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := GetUserFromContext(c)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
			c.Abort()
			return
		}

		if claims.IsSuperAdmin {
			c.Next()
			return
		}

		urlClientID := c.Param("id")
		if claims.ClientID == "" || claims.ClientID != urlClientID {
			logger.Warn("Client scope check failed",
				logger.String("user_id", claims.UserID),
				logger.String("jwt_client_id", claims.ClientID),
				logger.String("url_client_id", urlClientID),
			)
			response.Error(c, http.StatusForbidden, "Forbidden", "You cannot access this client's resources")
			c.Abort()
			return
		}

		c.Next()
	}
}
