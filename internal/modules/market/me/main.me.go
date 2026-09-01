// Package me implements GET /market/v1/me, the marketplace identity
// endpoint. There is no service layer: the repository's ownership lookup
// (FindByUserID) IS the business logic, so the handler calls the
// repository directly rather than through a service that would only
// forward the call.
package me

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/me/handler"
	"siakang-api/internal/modules/market/me/repository"
)

type Module struct {
	Handler *handler.Handler
}

func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	return &Module{Handler: handler.NewHandler(repo)}
}

// SetupRoutes mounts GET /me onto the group the caller passes — market's
// /market/v1 group, already behind JWTAuth().
func (m *Module) SetupRoutes(router *gin.RouterGroup) {
	router.GET("/me", m.Handler.GetMe)
}
