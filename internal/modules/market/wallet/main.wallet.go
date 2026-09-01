// Package wallet is the market domain's wallet submodule: two read-only
// endpoints over market.wallets and market.ledger_entries. There is no
// service layer — the handler calls the repository directly, since neither
// endpoint has business logic to hold between them.
package wallet

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/wallet/handler"
	"siakang-api/internal/modules/market/wallet/repository"
)

// Module holds the wallet submodule's handler.
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
	v1.GET("/wallet", m.Handler.GetWallet)
	v1.GET("/wallet/ledger", m.Handler.GetLedger)
}
