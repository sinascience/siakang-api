package middleware

import (
	"context"

	"github.com/gin-gonic/gin"

	jwtpkg "siakang-api/pkg/jwt"
)

// UserBranchResolver looks up the active branch IDs a user has access to.
// Implemented by core.branch.repository.UserBranchRepository.
type UserBranchResolver interface {
	FindBranchIDsByUser(ctx context.Context, userID string) ([]string, error)
}

// userBranchResolver is the global resolver wired from the router. nil means
// "branch scoping disabled" — no middleware enforcement happens even if
// BranchScope is in the route chain (safe default: callers still rely on
// CompanyContext).
var userBranchResolver UserBranchResolver

// SetUserBranchResolver wires the resolver from the router.
func SetUserBranchResolver(r UserBranchResolver) {
	userBranchResolver = r
}

// branchScopeKey is the gin context key where the resolved allowlist is
// stashed. A missing key means "no scoping applied" (super admin or user
// without any user_branches assignments).
const branchScopeKey = "allowed_branch_ids"

// BranchScope injects the caller's allowed branch IDs into the request
// context. Usage:
//
//	group.Use(middleware.JWTAuth(), middleware.CompanyContext(), middleware.BranchScope())
//
// Semantics:
//   - super_admin          → no entry set, callers see GetAllowedBranchIDs == nil
//   - user with 0 rows     → no entry set, nil (unrestricted — "belum dibatasi")
//   - user with ≥1 rows    → whitelist slice set, callers MUST apply the filter
//
// Handlers/repositories read the allowlist via GetAllowedBranchIDs(c). A nil
// return means "no scoping — show everything in tenant". A non-nil slice
// means "restrict to these branch_ids only". An empty non-nil slice is
// impossible (would have been treated as the 0-row case and not stored).
//
// If the resolver is not wired (SetUserBranchResolver never called), this
// middleware is a no-op. Safe to sprinkle on routes proactively.
func BranchScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userBranchResolver == nil {
			c.Next()
			return
		}
		claims, err := GetUserFromContext(c)
		if err != nil || claims == nil {
			// JWTAuth should have fired earlier and aborted; if we get here
			// without claims, don't block the request — let the missing auth
			// surface elsewhere.
			c.Next()
			return
		}
		if claims.HasRole(jwtpkg.RoleSuperAdmin) {
			c.Next()
			return
		}

		ids, err := userBranchResolver.FindBranchIDsByUser(c.Request.Context(), claims.UserID)
		if err != nil {
			// Fail-open: a DB hiccup here should not blanket-deny the whole
			// tenant. Downstream company scoping still applies. The error is
			// already logged inside the resolver.
			c.Next()
			return
		}
		if len(ids) > 0 {
			c.Set(branchScopeKey, ids)
		}
		c.Next()
	}
}

// GetAllowedBranchIDs returns the caller's branch allowlist, or nil if no
// scoping applies (super admin, no user_branches rows, or middleware not run).
// A nil return means "show everything visible under company scope".
func GetAllowedBranchIDs(c *gin.Context) []string {
	raw, ok := c.Get(branchScopeKey)
	if !ok {
		return nil
	}
	ids, _ := raw.([]string)
	return ids
}

// IsBranchAllowed is a convenience check for single-entity reads/writes.
// Returns true when there is no scoping OR when branchID is in the allowlist.
func IsBranchAllowed(c *gin.Context, branchID string) bool {
	allowed := GetAllowedBranchIDs(c)
	if allowed == nil {
		return true
	}
	for _, id := range allowed {
		if id == branchID {
			return true
		}
	}
	return false
}
