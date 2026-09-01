package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/market/me/dto"
	"siakang-api/internal/modules/market/me/repository"
	"siakang-api/internal/shared/response"
)

type Handler struct {
	repo *repository.Repository
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

// GetMe resolves the caller's marketplace identity: lapak is non-null iff a
// market.lapak_profiles row references the JWT's user_id, null for a
// customer. Reads user_id from the token only — this domain is not
// company-scoped (product ruling 2026-09-02).
func (h *Handler) GetMe(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	profile, err := h.repo.FindByUserID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to resolve marketplace identity", err.Error())
		return
	}

	resp := dto.MarketMeResponse{}
	if profile != nil {
		resp.Lapak = &dto.LapakProfileResponse{
			ID:          profile.ID,
			UserID:      profile.UserID,
			Name:        profile.Name,
			Description: profile.Description,
			Lat:         profile.Lat,
			Lng:         profile.Lng,
			Rating:      profile.Rating,
			IsAvailable: profile.IsAvailable,
		}
	}
	response.Success(c, http.StatusOK, "Marketplace identity retrieved successfully", resp)
}
