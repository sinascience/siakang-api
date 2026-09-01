package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"siakang-api/internal/modules/market/gig/domain"
	"siakang-api/internal/modules/market/gig/dto"
	"siakang-api/internal/modules/market/gig/repository"
	"siakang-api/internal/shared/response"
)

// Handler calls the repository directly — two read-only catalog endpoints
// with no business logic between them, same shape as product.
type Handler struct {
	repo *repository.Repository
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func toGigResponse(g domain.Gig) dto.GigResponse {
	tiers := make([]dto.GigTierResponse, 0, len(g.Tiers))
	for _, t := range g.Tiers {
		tiers = append(tiers, dto.GigTierResponse{
			ID:          t.ID,
			GigID:       t.GigID,
			Name:        t.Name,
			Description: t.Description,
			PriceIDR:    t.PriceIDR,
		})
	}
	return dto.GigResponse{
		ID:          g.ID,
		Title:       g.Title,
		Description: g.Description,
		ImageURL:    g.ImageURL,
		Lapak: dto.LapakSummaryResponse{
			ID:     g.LapakID,
			Name:   g.LapakName,
			Rating: g.LapakRating,
		},
		Tiers: tiers,
	}
}

// List handles GET /market/v1/gigs. Platform-wide catalog: every signed-in
// user sees every gig, no ownership filter — JWTAuth() alone (applied by
// main.market.go) is the entire authorization here.
func (h *Handler) List(c *gin.Context) {
	var params dto.GigQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	gigs, total, err := h.repo.List(c.Request.Context(), params.Page, params.Limit, params.Q)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list gigs", err.Error())
		return
	}

	items := make([]dto.GigResponse, 0, len(gigs))
	for _, g := range gigs {
		items = append(items, toGigResponse(g))
	}

	response.SuccessWithPagination(c, http.StatusOK, "Gigs retrieved successfully",
		items, params.Page, params.Limit, total)
}

// Get handles GET /market/v1/gigs/{id}. A malformed id gets the same 404 as
// a well-formed but unknown one — the contract's NotFound response
// deliberately keeps "no such resource" and "cannot name a resource"
// indistinguishable, rather than 500ing on input the client controls.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.Error(c, http.StatusNotFound, "Gig not found", "")
		return
	}

	gig, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrGigNotFound) {
			response.Error(c, http.StatusNotFound, "Gig not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get gig", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Gig retrieved successfully", toGigResponse(*gig))
}
