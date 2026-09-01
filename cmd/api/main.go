package main

import (
	"time"

	"siakang-api/internal/config"
	"siakang-api/internal/database"
	"siakang-api/internal/router"
	"siakang-api/pkg/jwt"
	"siakang-api/pkg/logger"

	"github.com/gin-gonic/gin"
)

func init() {
	// Force UTC timezone for the entire application
	// This ensures all time.Now() calls return UTC time
	time.Local = time.UTC
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	if err := logger.Initialize(cfg.Server.Env); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting Tuai API")

	// Validate JWT secret configuration
	if err := jwt.ValidateSecret(cfg.Server.Env); err != nil {
		logger.Fatal("JWT secret validation failed: " + err.Error())
	}
	logger.Info("JWT secret validation passed")

	// Initialize database
	db, err := database.New(cfg.Database.GetDSN())
	if err != nil {
		logger.Fatal("Failed to connect to database")
	}
	defer db.Close()

	logger.Info("Database connected successfully")

	// Initialize Gin router
	ginRouter := gin.New() // Use gin.New() instead of gin.Default() since we use custom middleware

	// Setup routes (all modules initialized inside router)
	router.Setup(ginRouter, db.Pool, cfg)

	// Start server. Binds SERVER_HOST (default 127.0.0.1) rather than every
	// interface — see ServerConfig.Host.
	serverAddr := cfg.Server.Host + ":" + cfg.Server.Port
	logger.Info("Starting server on " + serverAddr + " (Environment: " + cfg.Server.Env + ")")

	if err := ginRouter.Run(serverAddr); err != nil {
		logger.Fatal("Failed to start server")
	}
}
