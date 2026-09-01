// Package order is the market domain's order submodule: create, pay, list and
// read. It owns the sprint's money path — POST /orders/{id}/pay is the single
// place a wallet is charged for an order, reused unchanged by the flow-B
// upsell and both flow-C bid flows.
//
// It has a service layer where wallet and me do not, because there is real
// business logic to hold: one transaction covering payment, ledger, wallet,
// items, order status and the order's chat thread.
package order

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/order/handler"
	"siakang-api/internal/modules/market/order/repository"
	"siakang-api/internal/modules/market/order/service"
)

// Module holds the order submodule's handler.
type Module struct {
	Handler *handler.Handler
}

// Initialize wires repository, service and handler.
func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	return &Module{Handler: handler.NewHandler(service.NewService(repo))}
}

// SetupRoutes mounts onto the /market/v1 group, which main.market.go has
// already put JWTAuth() on. No CompanyContext, no RequirePermission:
// authorization in this domain is ownership, enforced in the repository's
// WHERE clauses (product ruling 2026-09-02).
func (m *Module) SetupRoutes(v1 *gin.RouterGroup) {
	v1.POST("/orders", m.Handler.CreateOrder)
	v1.GET("/orders", m.Handler.ListOrders)
	v1.GET("/orders/:id", m.Handler.GetOrder)
	v1.POST("/orders/:id/pay", m.Handler.PayOrder)
}
