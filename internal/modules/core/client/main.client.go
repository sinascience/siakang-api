package client

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/core/client/handler"
	"siakang-api/internal/modules/core/client/repository"
	"siakang-api/internal/modules/core/client/service"
	jwtpkg "siakang-api/pkg/jwt"
)

type Module struct {
	Handler    *handler.Handler
	Service    *service.Service
	Repository *repository.Repository
}

func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	hdlr := handler.NewHandler(svc)

	return &Module{
		Handler:    hdlr,
		Service:    svc,
		Repository: repo,
	}
}

// SetupRoutes mounts the super_admin-only clients admin endpoints.
// There is intentionally no POST / DELETE — clients are created
// automatically at signup and retired via a separate ownership flow.
func (m *Module) SetupRoutes(router *gin.RouterGroup) {
	admin := router.Group("/admin/clients")
	admin.Use(middleware.JWTAuth())
	admin.Use(middleware.RequireRole(jwtpkg.RoleSuperAdmin))
	{
		admin.GET("", m.Handler.List)
		admin.GET("/:id", m.Handler.GetByID)
		admin.PUT("/:id", m.Handler.Update)
	}
}
