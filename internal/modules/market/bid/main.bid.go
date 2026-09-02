// Package bid is the market domain's flow-C submodule: automatic matching and
// manual bidding. One resource, two modes — POST /market/v1/bids serves both,
// and the list, detail and offer endpoints are shared.
//
// It has a service layer for the same reason order does: charging a fee before
// a search runs, refunding it in the same transaction when the search finds
// nobody, and turning an award into a fee plus a tracked order plus a chat
// thread are all real business logic, and none of it belongs in a handler.
package bid

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/bid/handler"
	"siakang-api/internal/modules/market/bid/repository"
	"siakang-api/internal/modules/market/bid/service"
)

// Module holds the bid submodule's handler.
type Module struct {
	Handler *handler.Handler
}

// Initialize wires repository, service and handler. No background half: the
// contract makes matching synchronous inside POST /bids, so there is nothing
// here for a sweeper or a queue to do.
func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	return &Module{Handler: handler.NewHandler(service.NewService(repo))}
}

// SetupRoutes mounts onto the /market/v1 group, which main.market.go has
// already put JWTAuth() on. No CompanyContext and no RequirePermission:
// authorization in this domain is ownership and persona, enforced in the
// repository's WHERE clauses and the service's refusal paths (product ruling
// 2026-09-02).
func (m *Module) SetupRoutes(v1 *gin.RouterGroup) {
	v1.GET("/bid-categories", m.Handler.ListCategories)

	v1.GET("/bids", m.Handler.ListBids)
	v1.POST("/bids", m.Handler.CreateBid)
	v1.GET("/bids/:id", m.Handler.GetBid)

	// Automatic: the customer confirms the proposal, the matched lapak accepts.
	v1.POST("/bids/:id/confirm", m.Handler.Confirm)
	v1.POST("/bids/:id/accept", m.Handler.Accept)

	// Manual: lapaks offer, the customer awards one.
	v1.GET("/bids/:id/offers", m.Handler.ListOffers)
	v1.POST("/bids/:id/offers", m.Handler.PlaceOffer)
	v1.POST("/bids/:id/offers/:offer_id/award", m.Handler.Award)
}
