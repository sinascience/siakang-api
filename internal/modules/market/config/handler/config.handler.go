package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/modules/market/config/dto"
	"siakang-api/internal/modules/market/config/repository"
	"siakang-api/internal/shared/response"
)

type Handler struct {
	repo *repository.Repository
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

// Get returns the seeded platform fees and auto-confirm window. Read-only:
// there is no admin UI to change them in sprint 1.
func (h *Handler) Get(c *gin.Context) {
	cfg, err := h.repo.Get(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to load platform configuration", err.Error())
		return
	}
	resp := dto.PlatformConfigResponse{
		BidAutoFeeIDR:           cfg.BidAutoFeeIDR,
		BidManualFeeIDR:         cfg.BidManualFeeIDR,
		OrderAutoConfirmSeconds: cfg.OrderAutoConfirmSeconds,
	}
	response.Success(c, http.StatusOK, "Platform configuration retrieved successfully", resp)
}
