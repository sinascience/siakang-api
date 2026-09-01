// Package config implements GET /market/v1/config, the seeded platform
// fees + auto-confirm window. There is no service layer: assembling the
// typed PlatformConfig from market.config's key/value rows IS the business
// logic (repository.Get), so the handler calls the repository directly
// rather than through a service that would only forward the call.
package config

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/config/handler"
	"siakang-api/internal/modules/market/config/repository"
)

type Module struct {
	Handler *handler.Handler
}

func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	return &Module{Handler: handler.NewHandler(repo)}
}

// SetupRoutes mounts GET /config onto the group the caller passes —
// market's /market/v1 group, already behind JWTAuth().
func (m *Module) SetupRoutes(router *gin.RouterGroup) {
	router.GET("/config", m.Handler.Get)
}
