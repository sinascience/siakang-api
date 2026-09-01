package approval

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/core/approval/handler"
	"siakang-api/internal/modules/core/approval/repository"
	"siakang-api/internal/modules/core/approval/service"
	roleRepo "siakang-api/internal/modules/core/role/repository"
)

// Module bundles exports for cross-module consumption.
//
// Finance feature modules depend on RequestService via Initialize.
type Module struct {
	ConfigHandler  *handler.ApprovalConfigHandler
	RequestHandler *handler.ApprovalRequestHandler

	ConfigService  *service.ApprovalConfigService
	RequestService *service.ApprovalRequestService

	ConfigRepository  *repository.ApprovalConfigRepository
	RequestRepository *repository.ApprovalRequestRepository
}

// Initialize wires the approval module. It needs RoleRepository so the
// request service can snapshot role names at submission time.
func Initialize(db *pgxpool.Pool, rRepo *roleRepo.RoleRepository) *Module {
	configRepo := repository.NewApprovalConfigRepository(db)
	requestRepo := repository.NewApprovalRequestRepository(db)

	configSvc := service.NewApprovalConfigService(configRepo)
	requestSvc := service.NewApprovalRequestService(configRepo, requestRepo, rRepo)

	return &Module{
		ConfigHandler:     handler.NewApprovalConfigHandler(configSvc),
		RequestHandler:    handler.NewApprovalRequestHandler(requestSvc),
		ConfigService:     configSvc,
		RequestService:    requestSvc,
		ConfigRepository:  configRepo,
		RequestRepository: requestRepo,
	}
}

// SetupRoutes registers HTTP endpoints under /core/v1.
func (m *Module) SetupRoutes(router *gin.RouterGroup) {
	configs := router.Group("/approval-configs")
	configs.Use(middleware.JWTAuth(), middleware.CompanyContext())
	{
		configs.GET("", m.ConfigHandler.GetAll)
		configs.GET("/:id", m.ConfigHandler.GetByID)
		configs.POST("", middleware.RequirePermission("approval_configs:create"), m.ConfigHandler.Create)
		configs.PUT("/:id", middleware.RequirePermission("approval_configs:update"), m.ConfigHandler.Update)
		configs.DELETE("/:id", middleware.RequirePermission("approval_configs:delete"), m.ConfigHandler.Delete)
	}

	requests := router.Group("/approval-requests")
	requests.Use(middleware.JWTAuth(), middleware.CompanyContext())
	{
		requests.GET("", m.RequestHandler.List)
		requests.GET("/inbox", m.RequestHandler.Inbox)
		requests.GET("/by-doc", m.RequestHandler.GetByDoc)
		requests.GET("/:id", m.RequestHandler.GetByID)
		requests.POST("/:id/approve", middleware.RequirePermission("approval_requests:approve"), m.RequestHandler.Approve)
		requests.POST("/:id/reject", middleware.RequirePermission("approval_requests:approve"), m.RequestHandler.Reject)
		requests.POST("/:id/cancel", m.RequestHandler.Cancel)
	}
}
