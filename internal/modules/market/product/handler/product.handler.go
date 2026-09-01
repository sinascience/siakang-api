package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"siakang-api/internal/modules/market/product/domain"
	"siakang-api/internal/modules/market/product/dto"
	"siakang-api/internal/modules/market/product/repository"
	"siakang-api/internal/shared/response"
)

// Handler calls the repository directly — two read-only catalog endpoints
// with no business logic between them, same shape as wallet and me.
type Handler struct {
	repo *repository.Repository
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func toProductResponse(p domain.Product) dto.ProductResponse {
	return dto.ProductResponse{
		ID:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		PriceIDR:    p.PriceIDR,
		ImageURL:    p.ImageURL,
		Lapak: dto.LapakSummaryResponse{
			ID:     p.LapakID,
			Name:   p.LapakName,
			Rating: p.LapakRating,
		},
	}
}

// List handles GET /market/v1/products. Platform-wide catalog: every
// signed-in user sees every product, no ownership filter — JWTAuth() alone
// (applied by main.market.go) is the entire authorization here.
func (h *Handler) List(c *gin.Context) {
	var params dto.ProductQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	products, total, err := h.repo.List(c.Request.Context(), params.Page, params.Limit, params.Q)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list products", err.Error())
		return
	}

	items := make([]dto.ProductResponse, 0, len(products))
	for _, p := range products {
		items = append(items, toProductResponse(p))
	}

	response.SuccessWithPagination(c, http.StatusOK, "Products retrieved successfully",
		items, params.Page, params.Limit, total)
}

// Get handles GET /market/v1/products/{id}. A malformed id gets the same
// 404 as a well-formed but unknown one — the contract's NotFound response
// deliberately keeps "no such resource" and "not a participant" (here:
// cannot even name a resource) indistinguishable, rather than 500ing on
// input the client controls.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.Error(c, http.StatusNotFound, "Product not found", "")
		return
	}

	product, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			response.Error(c, http.StatusNotFound, "Product not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get product", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Product retrieved successfully", toProductResponse(*product))
}
