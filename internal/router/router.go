package router

import (
	"context"
	"fmt"
	"siakang-api/internal/config"
	"siakang-api/internal/middleware"
	"time"

	// Core modules
	"siakang-api/internal/modules/core/api_key"
	"siakang-api/internal/modules/core/approval"
	"siakang-api/internal/modules/core/auth"
	"siakang-api/internal/modules/core/branch"
	"siakang-api/internal/modules/core/client"
	"siakang-api/internal/modules/core/company"
	"siakang-api/internal/modules/core/role"
	"siakang-api/internal/modules/core/translation_overrides"
	"siakang-api/internal/modules/core/user"

	// Market modules
	userRepo "siakang-api/internal/modules/core/user/repository"
	"siakang-api/internal/modules/market"

	"siakang-api/internal/shared/audit"
	"siakang-api/internal/shared/authz"
	sharedRedis "siakang-api/internal/shared/redis"

	pkgfirebase "siakang-api/pkg/firebase"
	"siakang-api/pkg/logger"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func Setup(router *gin.Engine, db *pgxpool.Pool, cfg *config.Config) {
	// Get logger instance
	log := logger.GetLogger()

	// ─── Redis & authz cache ────────────────────────────────────────
	// Redis backs the per-user permission cache (see
	// internal/shared/authz). It is load-bearing: RequirePermission
	// middleware refuses to answer without it, so a failed bootstrap
	// here is fatal rather than a silent degrade.
	redisClient, err := sharedRedis.New(context.Background(), cfg.Redis)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	log.Info("Redis connected",
		zap.String("addr", cfg.Redis.Host+":"+cfg.Redis.Port),
		zap.Int("db", cfg.Redis.DB),
		zap.Duration("permission_ttl", cfg.Redis.PermissionTTL),
	)

	// Ginzap middleware for logging HTTP requests
	router.Use(ginzap.Ginzap(log, time.RFC3339, true))

	// Recovery middleware with Zap (handles panics)
	router.Use(ginzap.RecoveryWithZap(log, true))

	// CORS middleware configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     devAndProdOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Tuai API is running",
		})
	})

	// Core v1 routes
	coreV1 := router.Group("/core/v1")
	{
		// Initialize and setup client module. Must come before auth so
		// that SignUp can provision a client for new registrants.
		clientModule := client.Initialize(db)
		clientModule.SetupRoutes(coreV1)

		// Initialize and setup auth module
		authModule := auth.Initialize(db, cfg)
		authModule.SetupRoutes(coreV1)

		// Wire the user_identities repo (used by Google sign-in to look
		// up / link Firebase identities to a core.users row).
		authModule.Service.SetUserIdentityRepo(userRepo.NewUserIdentityRepository(db))

		// Wire the Firebase Admin client when configured. If the
		// operator hasn't set FIREBASE_PROJECT_ID the verifier stays
		// nil; the /auth/google endpoint will then surface a 503
		// "Google sign-in is not configured" instead of crashing.
		if cfg.Firebase.ProjectID != "" {
			fbClient, err := pkgfirebase.New(context.Background(), cfg.Firebase.ProjectID, cfg.Firebase.CredentialsJSON)
			if err != nil {
				log.Fatal("Failed to initialize Firebase Admin", zap.Error(err))
			}
			authModule.Service.SetFirebaseVerifier(fbClient)
			log.Info("Firebase Admin initialized", zap.String("project_id", cfg.Firebase.ProjectID))
		} else {
			log.Warn("FIREBASE_PROJECT_ID not set — /auth/google endpoint disabled")
		}

		// Initialize and setup user module
		userModule := user.Initialize(db)
		userModule.SetupRoutes(coreV1)

		// Initialize and setup role module
		roleModule := role.Initialize(db)
		roleModule.SetupRoutes(coreV1)

		// Wire the permission cache now that both Redis and the role repo
		// exist. The role repo acts as the authoritative fetcher on cache
		// miss; role service uses the same cache to invalidate stale
		// entries when role permissions or assignments change; auth service
		// uses it to serve /auth/me without hitting the DB on every call.
		authzService := authz.NewService(redisClient, roleModule.Repository, cfg.Redis.PermissionTTL)
		middleware.SetAuthzService(authzService)
		roleModule.Service.SetPermissionCacheInvalidator(authzService)
		authModule.Service.SetPermissionReader(authzService)

		// Initialize and setup company module
		companyModule := company.Initialize(db)
		companyModule.SetupRoutes(coreV1)
		companyModule.SetupUserCompanyRoutes(coreV1)

		// Initialize and setup branch module
		branchModule := branch.Initialize(db)
		branchModule.SetupRoutes(coreV1)
		branchModule.SetupUserBranchRoutes(coreV1)

		// Wire up auth service dependencies for company operations
		authModule.Service.SetCompanyUserRepo(companyModule.UserRepository)
		authModule.Service.SetCompanyRepo(companyModule.Repository)
		authModule.Service.SetRoleRepo(roleModule.Repository)
		authModule.Service.SetBranchRepo(branchModule.Repository)
		authModule.Service.SetClientService(clientModule.Service)

		// Wire the client lookup into company service so Create can
		// resolve client_id for owners who have no primary company yet.
		companyModule.Service.SetClientLookup(clientModule.Repository)

		// Wire branch repo into company service so Create seeds a default branch
		companyModule.Service.SetBranchRepo(branchModule.Repository)

		// Wire default admin role so the creator gets full company-scoped
		// permissions out of the box (mirrors SignUp).
		companyModule.Service.SetDefaultAdminRoleID(cfg.Auth.DefaultAdminRoleID)

		// Wire the company-scope resolver into the user-branch service so
		// non-super-admin callers can only assign branches inside their
		// company subtree.
		branchModule.UserBranchService.SetScopeResolver(companyModule.Repository)

		// Wire the company membership lookup into the branch service so
		// /branches/by-companies can resolve non-super_admin callers'
		// allowed companies from core.company_users.
		branchModule.Service.SetCompanyMembershipLookup(companyModule.UserRepository)

		// Wire up user service dependencies for company sync on create
		userModule.Service.SetCompanySyncer(companyModule.Service)
		userModule.Service.SetBranchSyncer(branchModule.UserBranchService)
		userModule.Service.SetRoleLookup(roleModule.Repository)

		// Wire the live company-context verifier so that any endpoint guarded
		// by middleware.CompanyContext rejects stale JWT company claims (e.g.
		// user removed from the company, company soft-deleted/deactivated).
		middleware.SetCompanyContextVerifier(companyModule.UserRepository)

		// User module routes only run JWTAuth (not CompanyContext) because
		// super_admin paths must remain accessible without a tenant. Inject
		// the verifier directly into the user handler so its inline tenant
		// scope check enforces the same staleness guarantees.
		userModule.Handler.SetCompanyVerifier(companyModule.UserRepository)

		// Wire the branch-scope resolver so middleware.BranchScope() can
		// narrow branches reads to the caller's user_branches set.
		middleware.SetUserBranchResolver(branchModule.UserBranchRepository)

		// Initialize and setup API key module
		apiKeyModule := api_key.Initialize(db, roleModule.Repository)
		apiKeyModule.SetupRoutes(coreV1)

		// Wire up API key validator for middleware
		middleware.SetApiKeyValidator(apiKeyModule.Service)

		// Initialize and setup Approval module (depends on role repo for
		// snapshotting role names at submission time).
		approvalModule := approval.Initialize(db, roleModule.Repository)
		approvalModule.SetupRoutes(coreV1)

		// Initialize and setup Translation Overrides module. Exposes both
		// an unauthenticated bootstrap endpoint (resolves client by slug)
		// and super_admin CRUD nested under /admin/clients/:client_id.
		translationOverridesModule := translation_overrides.Initialize(db, clientModule.Service)
		translationOverridesModule.SetupRoutes(coreV1)
	}

	// Shared audit module (polymorphic, used by any feature that wants
	// per-entity history). Initialized after the core group so its routes
	// mount under /core/v1.
	auditModule := audit.Initialize(db)
	auditModule.SetupRoutes(coreV1)

	// ─── Market domain (SIAKANG marketplace) ────────────────────────
	// Mounted at /market/v1, outside the core group: marketplace routes
	// run JWTAuth() only — no CompanyContext(), no RequirePermission() —
	// and no market table is company-scoped (product ruling 2026-09-02,
	// see docs/architecture/market-tenancy-deviation.md).
	//
	// Marketplace submodules register inside market.Module, NOT here, so
	// that adding a feature touches one market-owned file instead of this
	// router that every module shares.
	marketModule := market.Initialize(db)
	marketModule.SetupRoutes(router.Group(""))

	log.Info("Routes setup completed", zap.Int("routes", len(router.Routes())))
}

// devAndProdOrigins is the CORS allow-list.
//
// The localhost 3000-3019 range exists because this pipeline fans work out to
// parallel agent worktrees, each needing its own dev-server port. An origin
// missing from this list does NOT surface as a CORS error: the browser reports
// an opaque "Network Error" that reads like a broken frontend. 3002 was absent
// during phase 4 and escaped notice only because that agent's work was
// mocks-only; 3005 cost a debugging cycle. Twenty slots is deliberately more
// than we expect to use, so the list stops being something anyone maintains.
//
// It is written as an explicit range on purpose. The tempting alternative — an
// AllowOriginFunc matching any http://localhost:<port> — must NOT ship: with
// AllowCredentials true, it would make any process able to bind a localhost
// port on a deployed host a trusted origin. Every entry here visibly says
// localhost, so this cannot leak into production by accident.
//
// 5173 is the owner's manual runs; 8081 is QA. Do not remove either.
func devAndProdOrigins() []string {
	origins := []string{"http://localhost:5173", "http://localhost:8081"}
	for port := 3000; port <= 3019; port++ {
		origins = append(origins, fmt.Sprintf("http://localhost:%d", port))
	}
	return append(origins,
		"https://app.tuai.id",
		"https://jesuit.venturo.pro",
		"https://skeleton.venturo.id",
	)
}
