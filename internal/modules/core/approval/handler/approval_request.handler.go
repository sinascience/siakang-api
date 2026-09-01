package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/core/approval/dto"
	"siakang-api/internal/modules/core/approval/service"
	"siakang-api/internal/shared/response"
)

type ApprovalRequestHandler struct {
	svc *service.ApprovalRequestService
}

func NewApprovalRequestHandler(svc *service.ApprovalRequestService) *ApprovalRequestHandler {
	return &ApprovalRequestHandler{svc: svc}
}

func (h *ApprovalRequestHandler) List(c *gin.Context) {
	var params dto.ApprovalRequestQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	items, total, err := h.svc.List(c.Request.Context(), companyID, userID, &params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list approval requests", err.Error())
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, "Approval requests retrieved successfully",
		items, params.Page, params.Limit, total)
}

func (h *ApprovalRequestHandler) Inbox(c *gin.Context) {
	var params dto.ApprovalRequestQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}
	params.OnlyMine = true
	params.Status = "waiting"

	companyID := middleware.GetCompanyID(c)
	userID := middleware.MustGetUserID(c)

	items, total, err := h.svc.List(c.Request.Context(), companyID, userID, &params)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list inbox", err.Error())
		return
	}
	response.SuccessWithPagination(c, http.StatusOK, "Inbox retrieved successfully",
		items, params.Page, params.Limit, total)
}

func (h *ApprovalRequestHandler) GetByDoc(c *gin.Context) {
	var params dto.ApprovalByDocQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	companyID := middleware.GetCompanyID(c)

	result, err := h.svc.GetLatestByDoc(c.Request.Context(), companyID, params.ReffType, params.ReffID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get approval request", err.Error())
		return
	}
	if result == nil {
		response.Error(c, http.StatusNotFound, "No approval request found for this document", "")
		return
	}
	response.Success(c, http.StatusOK, "Approval request retrieved successfully", result)
}

func (h *ApprovalRequestHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "Request ID is required", "")
		return
	}

	result, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrRequestNotFound) {
			response.Error(c, http.StatusNotFound, "Approval request not found", "")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get approval request", err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Approval request retrieved successfully", result)
}

func (h *ApprovalRequestHandler) Approve(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "Request ID is required", "")
		return
	}

	var req dto.ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID := middleware.MustGetUserID(c)
	result, err := h.svc.Approve(c.Request.Context(), id, userID, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			response.Error(c, http.StatusNotFound, "Approval request not found", "")
		case errors.Is(err, service.ErrRequestNotWaiting):
			response.Error(c, http.StatusBadRequest, "Approval request is not in waiting state", "")
		case errors.Is(err, service.ErrNotEligibleApprover):
			response.Error(c, http.StatusForbidden, "You are not an eligible approver at the current level", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to approve", err.Error())
		}
		return
	}
	response.Success(c, http.StatusOK, "Approved successfully", result.Request)
}

func (h *ApprovalRequestHandler) Reject(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "Request ID is required", "")
		return
	}

	var req dto.RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID := middleware.MustGetUserID(c)
	result, err := h.svc.Reject(c.Request.Context(), id, userID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			response.Error(c, http.StatusNotFound, "Approval request not found", "")
		case errors.Is(err, service.ErrRequestNotWaiting):
			response.Error(c, http.StatusBadRequest, "Approval request is not in waiting state", "")
		case errors.Is(err, service.ErrNotEligibleApprover):
			response.Error(c, http.StatusForbidden, "You are not an eligible approver at the current level", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to reject", err.Error())
		}
		return
	}
	response.Success(c, http.StatusOK, "Rejected successfully", result.Request)
}

func (h *ApprovalRequestHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, http.StatusBadRequest, "Request ID is required", "")
		return
	}

	var req dto.CancelRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	userID := middleware.MustGetUserID(c)
	result, err := h.svc.Cancel(c.Request.Context(), id, userID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			response.Error(c, http.StatusNotFound, "Approval request not found", "")
		case errors.Is(err, service.ErrRequestNotWaiting):
			response.Error(c, http.StatusBadRequest, "Approval request is not in waiting state", "")
		case errors.Is(err, service.ErrCannotCancel):
			response.Error(c, http.StatusForbidden, "Only the submitter can cancel a waiting request", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to cancel", err.Error())
		}
		return
	}
	response.Success(c, http.StatusOK, "Cancelled successfully", result)
}
