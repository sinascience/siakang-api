package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/market/chat/dto"
	"siakang-api/internal/modules/market/chat/repository"
	"siakang-api/internal/modules/market/chat/service"
	"siakang-api/internal/shared/response"
	jwtpkg "siakang-api/pkg/jwt"
	"siakang-api/pkg/logger"
)

const (
	// heartbeatInterval is the `: ping` cadence. An idle event stream is
	// dropped silently by proxies and by the browser, and criterion 6 then
	// goes flaky and looks like an FE bug. The plan allows up to 25s; 20
	// leaves margin for one slow hop.
	heartbeatInterval = 20 * time.Second

	// retryMillis is emitted once on connect so EventSource's reconnect
	// backoff is defined by us rather than left to the user agent.
	retryMillis = 3000
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// ListThreads handles GET /market/v1/chat/threads — the caller's threads, most
// recently active first. Both personas list their own; whose threads they are
// is the server's decision, so there is no query parameter that chooses.
func (h *Handler) ListThreads(c *gin.Context) {
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

	threads, total, err := h.svc.ListThreads(c.Request.Context(), userID, params.Page, params.Limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list chat threads", err.Error())
		return
	}

	items := make([]dto.ThreadResponse, 0, len(threads))
	for _, t := range threads {
		items = append(items, dto.NewThreadResponse(t))
	}

	response.SuccessWithPagination(c, http.StatusOK, "Chat threads retrieved successfully",
		items, params.Page, params.Limit, total)
}

// ListMessages handles GET /market/v1/chat/threads/{id}/messages — history,
// newest first. FE loads this once and then appends from the stream.
func (h *Handler) ListMessages(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var param dto.ThreadIDParam
	if err := c.ShouldBindUri(&param); err != nil {
		notFound(c)
		return
	}

	var params dto.ListQueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	messages, total, err := h.svc.ListMessages(c.Request.Context(), userID, param.ID, params.Page, params.Limit)
	if err != nil {
		if errors.Is(err, repository.ErrThreadNotFound) {
			notFound(c)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to list chat messages", err.Error())
		return
	}

	items := make([]dto.MessageResponse, 0, len(messages))
	for _, m := range messages {
		items = append(items, dto.NewMessageResponse(m))
	}

	response.SuccessWithPagination(c, http.StatusOK, "Chat messages retrieved successfully",
		items, params.Page, params.Limit, total)
}

// SendMessage handles POST /market/v1/chat/threads/{id}/messages. The sender is
// the JWT's user id — never a body field, so no request shape posts a message
// in someone else's name.
func (h *Handler) SendMessage(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}

	var param dto.ThreadIDParam
	if err := c.ShouldBindUri(&param); err != nil {
		notFound(c)
		return
	}

	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
			map[string][]string{"body": {err.Error()}})
		return
	}

	// `required` rejects "" but not "   ", and the schema's
	// CHECK (LENGTH(TRIM(body)) > 0) would then turn a whitespace-only body
	// into a constraint violation and a 500. Reject it here as validation,
	// and store the trimmed text so the row and the check agree.
	body := strings.TrimSpace(req.Body)
	if body == "" {
		response.ValidationError(c, http.StatusUnprocessableEntity, "Validation failed",
			map[string][]string{"body": {"body must not be blank"}})
		return
	}

	msg, err := h.svc.Send(c.Request.Context(), userID, param.ID, body)
	if err != nil {
		if errors.Is(err, repository.ErrThreadNotFound) {
			notFound(c)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to send message", err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Message sent successfully", dto.NewMessageResponse(*msg))
}

// Stream handles GET /market/v1/chat/threads/{id}/stream — the Server-Sent
// Events endpoint.
//
// It is mounted on the ROOT group, outside /market/v1 and therefore outside
// JWTAuth(): EventSource cannot set an Authorization header, so the token
// arrives as ?token=<jwt> and the group middleware would reject the request
// before this ever ran. The validation below is the same jwtpkg.ParseToken the
// middleware calls — same signature check, same expiry, same errors.
func (h *Handler) Stream(c *gin.Context) {
	claims, err := jwtpkg.ParseToken(c.Query("token"))
	if err != nil {
		// Nothing has been written yet, so a bad token is an ordinary 401
		// JSON envelope. This is the last moment a status code can be chosen.
		logger.Warn("Chat stream rejected: invalid token", logger.Err(err))
		response.Error(c, http.StatusUnauthorized, "Unauthorized", tokenErrorMessage(err))
		return
	}

	var param dto.ThreadIDParam
	if err := c.ShouldBindUri(&param); err != nil {
		notFound(c)
		return
	}

	sub, unsubscribe, err := h.svc.Subscribe(c.Request.Context(), claims.UserID, param.ID)
	if err != nil {
		if errors.Is(err, repository.ErrThreadNotFound) {
			notFound(c)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to open chat stream", err.Error())
		return
	}
	// The subscriber is removed however this function returns — client gone,
	// token expired, or a failed write. A hub that only ever adds leaks a
	// channel per reconnect, and EventSource reconnects on every blip.
	defer func() {
		remaining := unsubscribe()
		logger.Info("Chat stream closed",
			logger.String("thread_id", param.ID),
			logger.String("user_id", claims.UserID),
			logger.Int("subscribers_remaining", remaining),
		)
	}()

	logger.Info("Chat stream opened",
		logger.String("thread_id", param.ID),
		logger.String("user_id", claims.UserID),
		logger.Int("subscribers", h.svc.Hub().Count(param.ID)),
	)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// Tells nginx not to buffer this response. Without it a proxy holds the
	// frames and the browser sees nothing until the buffer fills.
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	if !send(c, "retry: %d\n\n", retryMillis) {
		return
	}

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	// A mid-stream 401 is physically impossible — the 200 and the
	// content-type are already on the wire and cannot be retracted. So the
	// stream says so in-band instead, at the exp the token itself carries.
	// A nil channel blocks forever in select, which is the right behaviour
	// for a token with no expiry.
	var expired <-chan time.Time
	if claims.ExpiresAt != nil {
		timer := time.NewTimer(time.Until(claims.ExpiresAt.Time))
		defer timer.Stop()
		expired = timer.C
	}

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-sub:
			payload, err := json.Marshal(dto.NewMessageResponse(msg))
			if err != nil {
				logger.Error("Failed to encode chat message frame", logger.Err(err))
				continue
			}
			if !send(c, "event: chat.message\ndata: %s\n\n", payload) {
				return
			}

		case <-heartbeat.C:
			if !send(c, ": ping\n\n") {
				return
			}

		case <-expired:
			logger.Info("Chat stream token expired",
				logger.String("thread_id", param.ID),
				logger.String("user_id", claims.UserID),
			)
			send(c, "event: auth.expired\ndata: {}\n\n")
			return
		}
	}
}

// send writes one frame and flushes it. Gin buffers by default, so without the
// explicit flush the browser receives nothing until the buffer fills — the
// classic "SSE delivers nothing" bug. False means the socket is gone and the
// stream should end.
func send(c *gin.Context, format string, args ...any) bool {
	if _, err := fmt.Fprintf(c.Writer, format, args...); err != nil {
		logger.Warn("Chat stream write failed", logger.Err(err))
		return false
	}
	c.Writer.Flush()
	return true
}

// notFound is the single answer for a bad id, a missing thread and a caller
// who is not a participant. They are deliberately indistinguishable: a 403
// would confirm the thread exists.
func notFound(c *gin.Context) {
	response.Error(c, http.StatusNotFound, "Chat thread not found", "no such chat thread")
}

// tokenErrorMessage mirrors the wording JWTAuth() uses, so a stream rejected
// before it opens is indistinguishable from any other 401 in the API.
func tokenErrorMessage(err error) string {
	switch {
	case errors.Is(err, jwtpkg.ErrExpiredToken):
		return "Token has expired"
	case errors.Is(err, jwtpkg.ErrInvalidSignature):
		return "Invalid token signature"
	case errors.Is(err, jwtpkg.ErrTokenNotProvided):
		return "token query parameter required"
	default:
		return "Invalid token"
	}
}
