// Package market is the SIAKANG marketplace domain: catalog, orders,
// payments, bids and chat.
//
// It is deliberately NOT company-scoped. Where the core modules chain
// JWTAuth() → CompanyContext() → RequirePermission() and filter every query
// by company_id, /market/v1/* runs JWTAuth() only and authorizes by
// ownership in repository WHERE clauses. Product ruling 2026-09-02; the
// reasoning is in docs/architecture/market-tenancy-deviation.md.
//
// This file is the domain's single registration point. Submodules are wired
// here rather than in internal/router/router.go, so that adding a market
// feature touches one market-owned file instead of the router every module
// in the codebase shares.
package market

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/middleware"
	"siakang-api/internal/modules/market/chat"
	"siakang-api/internal/modules/market/config"
	"siakang-api/internal/modules/market/gig"
	"siakang-api/internal/modules/market/me"
	"siakang-api/internal/modules/market/order"
	"siakang-api/internal/modules/market/product"
	"siakang-api/internal/modules/market/wallet"
)

// Module holds the marketplace submodules. Each is initialized in
// Initialize and mounted in SetupRoutes.
type Module struct {
	db *pgxpool.Pool

	// MARKET SUBMODULE FIELDS — add yours here, one line.
	Me      *me.Module
	Config  *config.Module
	Wallet  *wallet.Module
	Product *product.Module
	Order   *order.Module
	Chat    *chat.Module
	Gig     *gig.Module
}

// Initialize builds every marketplace submodule.
func Initialize(db *pgxpool.Pool) *Module {
	m := &Module{db: db}

	// MARKET SUBMODULE INIT — add yours here.
	m.Me = me.Initialize(db)
	m.Config = config.Initialize(db)
	m.Wallet = wallet.Initialize(db)
	m.Product = product.Initialize(db)
	m.Order = order.Initialize(db)
	m.Chat = chat.Initialize(db)
	m.Gig = gig.Initialize(db)

	return m
}

// SetupRoutes mounts /market/v1. The caller passes the engine's root group.
//
// JWTAuth() is applied to the group, so every marketplace route requires a
// valid token and none of them require a company. The one exception is the
// chat SSE stream, which authenticates from a query parameter because
// EventSource cannot set an Authorization header — it registers itself
// outside this group.
func (m *Module) SetupRoutes(router *gin.RouterGroup) {
	v1 := router.Group("/market/v1")
	v1.Use(middleware.JWTAuth())
	{
		// MARKET SUBMODULE ROUTES — mount yours here, one line.
		m.Me.SetupRoutes(v1)
		m.Config.SetupRoutes(v1)
		m.Wallet.SetupRoutes(v1)
		m.Product.SetupRoutes(v1)
		m.Order.SetupRoutes(v1)
		m.Chat.SetupRoutes(v1)
		m.Gig.SetupRoutes(v1)
	}

	// Outside the group on purpose: the chat SSE stream takes its token from
	// a query parameter, so JWTAuth() would reject it before the handler ran.
	m.Chat.SetupStreamRoute(router)
}
