// Package gig is the market domain's read-only catalog submodule for flow
// B: list and detail over market.gigs, each row joined to its selling
// market.lapak_profiles row and its market.gig_tiers, price ascending. No
// service layer — same reasoning as product and wallet: two reads, no
// business logic between them, so a service would only forward the call.
package gig

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/gig/handler"
	"siakang-api/internal/modules/market/gig/repository"
)

// Module holds the gig submodule's handler.
type Module struct {
	Handler *handler.Handler
}

// Initialize wires the repository and handler.
func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	return &Module{Handler: handler.NewHandler(repo)}
}

// SetupRoutes mounts onto the /market/v1 group, which main.market.go has
// already put JWTAuth() on.
func (m *Module) SetupRoutes(v1 *gin.RouterGroup) {
	v1.GET("/gigs", m.Handler.List)
	v1.GET("/gigs/:id", m.Handler.Get)
}
