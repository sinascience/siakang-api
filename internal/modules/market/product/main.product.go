// Package product is the market domain's read-only catalog submodule for
// flow A: list and detail over market.products, each row joined to its
// selling market.lapak_profiles row. No service layer — same reasoning as
// wallet: two reads, no business logic between them, so a service would
// only forward the call.
package product

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/product/handler"
	"siakang-api/internal/modules/market/product/repository"
)

// Module holds the product submodule's handler.
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
	v1.GET("/products", m.Handler.List)
	v1.GET("/products/:id", m.Handler.Get)
}
