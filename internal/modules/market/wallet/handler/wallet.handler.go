package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/market/wallet/dto"
	"siakang-api/internal/modules/market/wallet/repository"
	"siakang-api/internal/shared/response"
	"siakang-api/pkg/logger"
)

// Handler calls the repository directly — two reads with no business logic
// between them, so a service layer here would only forward the call.
type Handler struct {
	repo *repository.Repository
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

// GetWallet handles GET /market/v1/wallet. The wallet returned is always the
// caller's own: userID comes from the JWT claims set by JWTAuth(), never
// from a query, path, or body param, so there is no request shape that can
// read someone else's balance.
func (h *Handler) GetWallet(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	wallet, err := h.repo.GetWallet(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrWalletNotFound) {
			// The contract defines only 200/401 for this endpoint — no 404.
			// "No wallet row" and "no money" are the same fact to the
			// caller (a self-registered core.users row has no market
			// provisioning yet), so answer 200 with a zero balance. The
			// warning keeps a genuine provisioning gap visible in the logs
			// instead of silently passing as a normal empty wallet.
			logger.Warn("Wallet row missing for user, returning zero balance", logger.String("user_id", userID))
			response.Success(c, http.StatusOK, "Wallet retrieved successfully", dto.WalletResponse{
				UserID:     userID,
				BalanceIDR: 0,
			})
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get wallet", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Wallet retrieved successfully", dto.WalletResponse{
		UserID:     wallet.UserID,
		BalanceIDR: wallet.BalanceIDR,
	})
}

// GetLedger handles GET /market/v1/wallet/ledger — same own-wallet-only
// scoping as GetWallet, newest first, paginated per the contract.
func (h *Handler) GetLedger(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var params dto.LedgerQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	entries, total, err := h.repo.GetLedger(c.Request.Context(), userID, params.Page, params.Limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get ledger", err.Error())
		return
	}

	items := make([]dto.LedgerEntryResponse, 0, len(entries))
	for _, e := range entries {
		items = append(items, dto.LedgerEntryResponse{
			ID:              e.ID,
			Type:            e.Type,
			AmountIDR:       e.AmountIDR,
			BalanceAfterIDR: e.BalanceAfterIDR,
			OrderID:         e.OrderID,
			BidID:           e.BidID,
			Note:            e.Note,
			CreatedAt:       e.CreatedAt,
		})
	}

	response.SuccessWithPagination(c, http.StatusOK, "Ledger retrieved successfully",
		items, params.Page, params.Limit, total)
}
