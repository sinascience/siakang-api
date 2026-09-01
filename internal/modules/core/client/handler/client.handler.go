package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/core/client/dto"
	"siakang-api/internal/modules/core/client/service"
	"siakang-api/internal/shared/response"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(c *gin.Context) {
	result, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list clients", err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Clients retrieved successfully", result)
}

func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")
	client, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	resp := service.ResponseFromDomain(client)
	response.Success(c, http.StatusOK, "Client retrieved successfully", resp)
}

func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	actorID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	result, err := h.svc.Update(c.Request.Context(), id, &req, actorID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Client updated successfully", result)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		response.Error(c, http.StatusNotFound, "Client not found", "")
	case errors.Is(err, service.ErrInvalidSlug):
		response.Error(c, http.StatusBadRequest, "Invalid slug", err.Error())
	case errors.Is(err, service.ErrSlugTaken):
		response.Error(c, http.StatusConflict, "Slug is already taken", "")
	default:
		response.Error(c, http.StatusInternalServerError, "Internal server error", err.Error())
	}
}
