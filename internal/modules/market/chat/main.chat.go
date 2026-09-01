// Package chat is the market domain's customer↔lapak conversation: thread
// list, history, send, and the Server-Sent Events stream that delivers a
// message to the other party without a reload.
//
// Threads are created by the server, never here: BE-05's pay transaction opens
// one per order (contract amendment v1.0.2). There is deliberately no
// thread-creation endpoint.
package chat

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/chat/handler"
	"siakang-api/internal/modules/market/chat/repository"
	"siakang-api/internal/modules/market/chat/service"
)

// Module holds the chat submodule's handler. The fan-out hub lives inside the
// service and is created once, in Initialize — a hub per request would deliver
// nothing to anyone.
type Module struct {
	Handler *handler.Handler
}

// Initialize wires repository, service and handler.
func Initialize(db *pgxpool.Pool) *Module {
	repo := repository.NewRepository(db)
	return &Module{Handler: handler.NewHandler(service.NewService(repo))}
}

// SetupRoutes mounts the JSON endpoints onto /market/v1, which main.market.go
// has already put JWTAuth() on. Authorization is thread participation,
// enforced in the repository's WHERE clauses.
func (m *Module) SetupRoutes(v1 *gin.RouterGroup) {
	v1.GET("/chat/threads", m.Handler.ListThreads)
	v1.GET("/chat/threads/:id/messages", m.Handler.ListMessages)
	v1.POST("/chat/threads/:id/messages", m.Handler.SendMessage)
}

// SetupStreamRoute mounts the SSE stream on the ROOT group, outside
// /market/v1 and therefore outside JWTAuth().
//
// EventSource cannot set an Authorization header, so the stream's token
// arrives as ?token=<jwt> and JWTAuth() would reject the request before the
// handler ran. The handler validates that token itself, through the same
// jwtpkg.ParseToken the middleware calls.
func (m *Module) SetupStreamRoute(root *gin.RouterGroup) {
	root.GET("/market/v1/chat/threads/:id/stream", m.Handler.Stream)
}
