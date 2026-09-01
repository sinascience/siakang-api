.PHONY: help migrate-up migrate-down migrate-create migrate-force migrate-version seed seed-core seed-market db-setup db-reset dev build run

help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev                     - Run with hot reload (Air)"
	@echo "  make build                   - Build application binary"
	@echo "  make run                     - Run application"
	@echo ""
	@echo "Database migrations:"
	@echo "  make migrate-up              - Run all pending migrations"
	@echo "  make migrate-down            - Rollback last migration"
	@echo "  make migrate-create NAME=xxx - Create new migration file"
	@echo "  make migrate-force V=xxx     - Force migration to specific version"
	@echo "  make migrate-version         - Show current migration version"
	@echo ""
	@echo "Seeding commands:"
	@echo "  make seed                    - Run all seeders"
	@echo "  make seed-core               - Run core module seeders only"
	@echo "  make seed-market             - Run market module seeders only"
	@echo ""
	@echo "Setup commands:"
	@echo "  make db-setup                - Run migrations + seeders (fresh setup)"
	@echo "  make db-reset                - Drop, migrate, and seed database"

# Database migrations
DB_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
MIGRATIONS_PATH=internal/database/migrations
SEEDERS_PATH=internal/database/seeders

# Default values from .env
include .env
export

migrate-up:
	@echo "Running migrations..."
	@if [ -d "$(MIGRATIONS_PATH)/core" ]; then \
		echo "Running core migrations..."; \
		migrate -path $(MIGRATIONS_PATH)/core -database "$(DB_URL)&x-migrations-table=schema_migrations_core" up; \
	fi

migrate-down:
	@if [ -z "$(MODULE)" ]; then echo "Error: MODULE is required. Usage: make migrate-down MODULE=core"; exit 1; fi
	@echo "Rolling back last migration for module $(MODULE)..."
	migrate -path $(MIGRATIONS_PATH)/$(MODULE) -database "$(DB_URL)&x-migrations-table=schema_migrations_$(MODULE)" down 1

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Error: NAME is required. Usage: make migrate-create NAME=your_migration_name MODULE=core"; exit 1; fi
	@if [ -z "$(MODULE)" ]; then echo "Error: MODULE is required. Available: core"; exit 1; fi
	@echo "Creating migration: $(NAME) in module $(MODULE)"
	migrate create -ext sql -dir $(MIGRATIONS_PATH)/$(MODULE) -seq $(NAME)

migrate-force:
	@if [ -z "$(V)" ]; then echo "Error: V is required. Usage: make migrate-force V=1 MODULE=core"; exit 1; fi
	@if [ -z "$(MODULE)" ]; then echo "Error: MODULE is required. Available: core"; exit 1; fi
	migrate -path $(MIGRATIONS_PATH)/$(MODULE) -database "$(DB_URL)&x-migrations-table=schema_migrations_$(MODULE)" force $(V)

migrate-version:
	@echo "Core module version:"
	@migrate -path $(MIGRATIONS_PATH)/core -database "$(DB_URL)&x-migrations-table=schema_migrations_core" version

# Database seeders
seed-core:
	@echo "Running core seeders..."
	@if [ -d "$(SEEDERS_PATH)/core" ]; then \
		for file in $(SEEDERS_PATH)/core/*.sql; do \
			if [ -f "$$file" ]; then \
				echo "Running seeder: $$(basename $$file)"; \
				PGPASSWORD=$(DB_PASSWORD) psql -v ON_ERROR_STOP=1 -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f $$file || exit 1; \
			fi; \
		done; \
	fi
	@echo "✅ Core seeders completed!"

seed-market:
	@echo "Running market seeders..."
	@if [ -d "$(SEEDERS_PATH)/market" ]; then \
		for file in $(SEEDERS_PATH)/market/*.sql; do \
			if [ -f "$$file" ]; then \
				echo "Running seeder: $$(basename $$file)"; \
				PGPASSWORD=$(DB_PASSWORD) psql -v ON_ERROR_STOP=1 -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f $$file || exit 1; \
			fi; \
		done; \
	fi
	@echo "✅ Market seeders completed!"

# Order matters: market seeders insert into core.users / core.roles /
# core.user_roles, so core must be seeded first.
seed: seed-core seed-market
	@echo "✅ All seeders completed!"

# Database setup commands
db-setup:
	@echo "🚀 Setting up database..."
	@make migrate-up
	@echo ""
	@make seed
	@echo "✅ Database setup completed!"

db-reset:
	@echo "⚠️  Resetting database (this will drop all data)..."
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME) WITH (FORCE);"
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_NAME);"
	@echo "Database dropped and recreated!"
	@make db-setup

# Development commands
build:
	@echo "🔨 Building application..."
	@go build -o bin/tuai-api cmd/api/main.go
	@echo "✅ Build completed: bin/tuai-api"

run:
	@echo "🚀 Running application..."
	@go run cmd/api/main.go

dev:
	@echo "🔥 Starting development server with hot reload..."
	@~/go/bin/air
