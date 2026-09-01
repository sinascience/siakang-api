package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/market/order/dto"
	"siakang-api/internal/modules/market/order/repository"
	"siakang-api/internal/modules/market/order/service"
	"siakang-api/internal/shared/response"
)

// Handler is thin on purpose: bind, call the service, map an error to a status
// code. Everything that decides anything about money lives in the service.
type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// CreateOrder handles POST /market/v1/orders.
//
// The customer is the JWT's user id — never a body field, so there is no
// request shape that places an order in someone else's name.
func (h *Handler) CreateOrder(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var req dto.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
			map[string][]string{"detail": {err.Error()}})
		return
	}

	// "Exactly one of product_id / gig_tier_id" is a rule `binding` has no tag
	// for, so it is checked here and reported against the two fields it is
	// about.
	if (req.ProductID == "") == (req.GigTierID == "") {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
			map[string][]string{
				"product_id":  {"exactly one of product_id or gig_tier_id is required"},
				"gig_tier_id": {"exactly one of product_id or gig_tier_id is required"},
			})
		return
	}

	quantity := 1
	if req.Quantity != nil {
		quantity = *req.Quantity
	}

	order, err := h.svc.Create(c.Request.Context(), userID, req.ProductID, req.GigTierID, quantity)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLapakCannotOrder):
			response.Error(c, http.StatusForbidden, "Only customers can create orders", err.Error())
		case errors.Is(err, service.ErrGigNotSupported):
			response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
				map[string][]string{"gig_tier_id": {err.Error()}})
		case errors.Is(err, repository.ErrProductNotFound):
			response.Error(c, http.StatusNotFound, "Product not found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to create order", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "Order created successfully", dto.NewOrderResponse(order))
}

// GetOrder handles GET /market/v1/orders/{id}. Visible to the order's customer
// and its lapak; everybody else gets 404, not 403 — a 403 would confirm the
// order exists.
func (h *Handler) GetOrder(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var param dto.OrderIDParam
	if err := c.ShouldBindUri(&param); err != nil {
		// A malformed id names no row, so it gets the same answer a stranger
		// gets rather than a shape-revealing 400.
		response.Error(c, http.StatusNotFound, "Order not found", "no such order")
		return
	}

	order, err := h.svc.Get(c.Request.Context(), userID, param.ID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			response.Error(c, http.StatusNotFound, "Order not found", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get order", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Order retrieved successfully", dto.NewOrderResponse(order))
}

// PayOrder handles POST /market/v1/orders/{id}/pay. There is no request body:
// the amount is the order's outstanding total, computed server-side.
func (h *Handler) PayOrder(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var param dto.OrderIDParam
	if err := c.ShouldBindUri(&param); err != nil {
		response.Error(c, http.StatusNotFound, "Order not found", "no such order")
		return
	}

	result, err := h.svc.Pay(c.Request.Context(), userID, param.ID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrOrderNotFound):
			response.Error(c, http.StatusNotFound, "Order not found", err.Error())
		case errors.Is(err, service.ErrInsufficientBalance):
			// 402, per the contract. 422 stays validation-only.
			response.Error(c, http.StatusPaymentRequired, "Insufficient wallet balance", err.Error())
		case errors.Is(err, service.ErrNothingOutstanding):
			response.Error(c, http.StatusConflict, "Nothing outstanding to pay on this order", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to pay order", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Order paid successfully", dto.PayResultResponse{
		Order:            dto.NewOrderResponse(result.Order),
		Payment:          dto.NewPaymentResponse(*result.Payment),
		WalletBalanceIDR: result.WalletBalanceIDR,
	})
}

// ListOrders handles GET /market/v1/orders. Persona scoping is the server's,
// so there is no query parameter that chooses whose orders come back.
func (h *Handler) ListOrders(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var params dto.ListQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	orders, counts, total, err := h.svc.List(c.Request.Context(), userID, params.Status, params.Page, params.Limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list orders", err.Error())
		return
	}

	items := make([]dto.OrderResponse, 0, len(orders))
	for _, o := range orders {
		items = append(items, dto.NewOrderResponse(o))
	}

	// counts covers every status the caller has, filter or no filter, so one
	// request drives all the tab badges.
	response.SuccessWithPaginationAndCounts(c, http.StatusOK, "Orders retrieved successfully",
		items, params.Page, params.Limit, total, counts)
}
