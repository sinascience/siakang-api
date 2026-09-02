package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/market/bid/domain"
	"siakang-api/internal/modules/market/bid/dto"
	"siakang-api/internal/modules/market/bid/repository"
	"siakang-api/internal/modules/market/bid/service"
	"siakang-api/internal/shared/response"
)

// Handler is thin on purpose: bind, call the service, map an error to a status
// code. Everything that decides anything about money or matching lives in the
// service.
type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// ListCategories handles GET /market/v1/bid-categories. Not paginated: the set
// is seeded and admin-predeclared.
func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list bid categories", err.Error())
		return
	}

	items := make([]dto.CategoryResponse, 0, len(categories))
	for i := range categories {
		items = append(items, dto.NewCategoryResponse(&categories[i]))
	}
	response.Success(c, http.StatusOK, "Bid categories retrieved successfully", items)
}

// CreateBid handles POST /market/v1/bids, both modes.
//
// The customer is the JWT's user id — never a body field, so no request shape
// posts a bid in someone else's name.
func (h *Handler) CreateBid(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var req dto.CreateBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
			map[string][]string{"detail": {err.Error()}})
		return
	}

	// The per-mode requirements. `binding` has no tag for "required when mode
	// is auto", and a 422 is worth reporting against the fields it is about.
	fieldErrors := map[string][]string{}
	if req.Mode == domain.ModeAuto {
		if req.Lat == nil {
			fieldErrors["lat"] = []string{"lat is required for an automatic bid"}
		}
		if req.Lng == nil {
			fieldErrors["lng"] = []string{"lng is required for an automatic bid"}
		}
	}
	if req.Mode == domain.ModeManual && req.Title == "" {
		fieldErrors["title"] = []string{"title is required for a manual bid"}
	}
	if len(fieldErrors) > 0 {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed", fieldErrors)
		return
	}

	bid, err := h.svc.Create(c.Request.Context(), userID, req.Mode, req.CategoryID,
		req.Title, req.Description, req.BudgetIDR, req.Lat, req.Lng)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLapakCannotBid):
			response.Error(c, http.StatusForbidden, "Only customers can create bids", err.Error())
		case errors.Is(err, service.ErrInsufficientBalance):
			// 402, per the contract: nothing written, and no matching run.
			response.Error(c, http.StatusPaymentRequired, "Insufficient wallet balance for the bid fee", err.Error())
		case errors.Is(err, repository.ErrCategoryNotFound):
			// A body field naming nothing is a validation failure, not a
			// missing endpoint resource.
			response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
				map[string][]string{"category_id": {err.Error()}})
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to create bid", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "Bid created successfully", dto.NewBidResponse(bid))
}

// ListBids handles GET /market/v1/bids. Persona scoping is the server's, so
// there is no query parameter that chooses whose bids come back.
func (h *Handler) ListBids(c *gin.Context) {
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

	bids, total, err := h.svc.List(c.Request.Context(), userID, params.Mode, params.Status, params.Page, params.Limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list bids", err.Error())
		return
	}

	items := make([]dto.BidResponse, 0, len(bids))
	for _, b := range bids {
		items = append(items, dto.NewBidResponse(b))
	}
	response.SuccessWithPagination(c, http.StatusOK, "Bids retrieved successfully",
		items, params.Page, params.Limit, total)
}

// GetBid handles GET /market/v1/bids/{id}. Visible to the bid's customer, its
// matched lapak, any lapak while a manual bid is open, and the lapak that
// offered on it. Everyone else gets 404 — a 403 would confirm it exists.
func (h *Handler) GetBid(c *gin.Context) {
	userID, param, ok := h.bidScope(c)
	if !ok {
		return
	}

	bid, err := h.svc.Get(c.Request.Context(), userID, param.ID)
	if err != nil {
		h.fail(c, err, "Failed to get bid")
		return
	}
	response.Success(c, http.StatusOK, "Bid retrieved successfully", dto.NewBidResponse(bid))
}

// Confirm handles POST /market/v1/bids/{id}/confirm — the customer accepting
// the proposed worker. No money moves; the fee was charged at creation.
func (h *Handler) Confirm(c *gin.Context) {
	userID, param, ok := h.bidScope(c)
	if !ok {
		return
	}

	bid, err := h.svc.Confirm(c.Request.Context(), userID, param.ID)
	if err != nil {
		h.fail(c, err, "Failed to confirm bid")
		return
	}
	response.Success(c, http.StatusOK, "Bid confirmed; awaiting the worker", dto.NewBidResponse(bid))
}

// Accept handles POST /market/v1/bids/{id}/accept — the matched lapak taking
// the job, which is what creates the tracked order and opens the chat thread.
func (h *Handler) Accept(c *gin.Context) {
	userID, param, ok := h.bidScope(c)
	if !ok {
		return
	}

	bid, err := h.svc.Accept(c.Request.Context(), userID, param.ID)
	if err != nil {
		h.fail(c, err, "Failed to accept bid")
		return
	}
	response.Success(c, http.StatusOK, "Bid accepted; tracked order created", dto.NewBidResponse(bid))
}

// ListOffers handles GET /market/v1/bids/{id}/offers. The customer sees every
// offer; a lapak sees their own.
func (h *Handler) ListOffers(c *gin.Context) {
	userID, param, ok := h.bidScope(c)
	if !ok {
		return
	}

	offers, err := h.svc.ListOffers(c.Request.Context(), userID, param.ID)
	if err != nil {
		h.fail(c, err, "Failed to list bid offers")
		return
	}

	items := make([]dto.OfferResponse, 0, len(offers))
	for _, o := range offers {
		items = append(items, dto.NewOfferResponse(o))
	}
	response.Success(c, http.StatusOK, "Bid offers retrieved successfully", items)
}

// PlaceOffer handles POST /market/v1/bids/{id}/offers.
//
// 201 on the first offer, 200 when this lapak's existing offer is replaced —
// one row per lapak per bid, so posting again is neither a duplicate nor a
// rejection (criterion BE-09.4).
func (h *Handler) PlaceOffer(c *gin.Context) {
	userID, param, ok := h.bidScope(c)
	if !ok {
		return
	}

	var req dto.CreateOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
			map[string][]string{"detail": {err.Error()}})
		return
	}

	offer, created, err := h.svc.PlaceOffer(c.Request.Context(), userID, param.ID, req.AmountIDR, req.Message)
	if err != nil {
		h.fail(c, err, "Failed to place offer")
		return
	}

	if created {
		response.Success(c, http.StatusCreated, "Offer placed successfully", dto.NewOfferResponse(offer))
		return
	}
	response.Success(c, http.StatusOK, "Offer replaced successfully", dto.NewOfferResponse(offer))
}

// Award handles POST /market/v1/bids/{id}/offers/{offer_id}/award — the
// customer picking a winner, which charges the manual fee and creates the
// tracked order in one transaction.
func (h *Handler) Award(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var param dto.AwardParam
	if err := c.ShouldBindUri(&param); err != nil {
		// A malformed bid id or offer id names no row, so it gets the same
		// answer a stranger gets rather than a shape-revealing 400 or a 500
		// from SQL.
		response.Error(c, http.StatusNotFound, "Bid not found", "no such bid")
		return
	}

	bid, err := h.svc.Award(c.Request.Context(), userID, param.ID, param.OfferID)
	if err != nil {
		h.fail(c, err, "Failed to award bid")
		return
	}
	response.Success(c, http.StatusOK, "Bid awarded; tracked order created", dto.NewBidResponse(bid))
}

// bidScope is the preamble every {id} endpoint shares: the caller from the
// JWT, and the path id validated before it can reach SQL. It returns ok=false
// having already written the response.
func (h *Handler) bidScope(c *gin.Context) (string, dto.BidIDParam, bool) {
	var param dto.BidIDParam

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return "", param, false
	}
	if err := c.ShouldBindUri(&param); err != nil {
		// A malformed (non-uuid) id cannot name a row: 404 envelope, never a
		// 500 from a failed cast in SQL.
		response.Error(c, http.StatusNotFound, "Bid not found", "no such bid")
		return "", param, false
	}
	return userID, param, true
}

// fail is the single error map for every bid endpoint, so the 403/404 line the
// contract draws per endpoint is decided in the service and merely rendered
// here. Two different services deciding it two ways is exactly the bug this
// avoids.
func (h *Handler) fail(c *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, repository.ErrBidNotFound):
		response.Error(c, http.StatusNotFound, "Bid not found", err.Error())
	case errors.Is(err, repository.ErrOfferNotFound):
		response.Error(c, http.StatusNotFound, "Offer not found", err.Error())
	case errors.Is(err, service.ErrNotCustomer):
		response.Error(c, http.StatusForbidden, "Caller is not this bid's customer", err.Error())
	case errors.Is(err, service.ErrNotMatchedLapak):
		response.Error(c, http.StatusForbidden, "Caller is not the matched lapak", err.Error())
	case errors.Is(err, service.ErrNotLapak):
		response.Error(c, http.StatusForbidden, "Only lapak accounts can place offers", err.Error())
	case errors.Is(err, service.ErrInsufficientBalance):
		response.Error(c, http.StatusPaymentRequired, "Insufficient wallet balance for the platform fee", err.Error())
	case errors.Is(err, service.ErrWrongStatus):
		response.Error(c, http.StatusConflict, "Bid is not in the expected status for this step", err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, message, err.Error())
	}
}
