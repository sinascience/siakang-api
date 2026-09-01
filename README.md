# Tuai Backend

Backend API untuk Tuai menggunakan Golang + Gin + PostgreSQL.

## Prerequisites

- Go 1.25+
- PostgreSQL 14+
- golang-migrate CLI (untuk menjalankan migration manual)
- air CLI (untuk hot reload development)

## Installation

### 1. Install golang-migrate CLI

```bash
# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/

# Windows
choco install golang-migrate
```

### 2. Install Air CLI

```bash
# Install air untuk hot reload development
go install github.com/air-verse/air@latest

# Verify installation
air -v
```

### 3. Setup Database

```bash
# Buat database PostgreSQL
createdb tuai

# Copy .env.example ke .env dan sesuaikan konfigurasi
cp .env.example .env
```

### 4. Install Dependencies

```bash
go mod download
```

## Database Migration

### Running Migrations

```bash
# Run all migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check migration version
make migrate-version
```

### Creating New Migration

```bash
# Buat migration baru untuk module tertentu
make migrate-create NAME=create_users_table MODULE=core
make migrate-create NAME=create_invoices_table MODULE=finance
make migrate-create NAME=create_employees_table MODULE=hr
make migrate-create NAME=create_products_table MODULE=inventory
```

### Force Migration Version (jika ada error)

```bash
# Force ke versi tertentu
make migrate-force V=1 MODULE=core
```

## Project Structure

```
├── cmd/api/                    # Application entry point
├── internal/
│   ├── config/                 # Configuration
│   ├── database/              # Database connection & migrations
│   ├── shared/                # Shared utilities
│   ├── core/                  # Core module (user, auth, role)
│   ├── finance/               # Finance module
│   ├── hr/                    # HR module
│   ├── inventory/             # Inventory module
│   └── router/                # Route handlers
└── pkg/                       # Public packages
```

## Running the Application

### Development Mode (with Hot Reload) 🔥

```bash
make dev
```

Server akan auto-reload setiap ada perubahan code!

### Manual Run

```bash
# Run directly
make run

# Or with go run
go run cmd/api/main.go
```

### Build & Run Production

```bash
# Build binary
make build

# Run binary
./bin/tuai-api
```

## Development

### Quick Start

1. Copy environment file

   ```bash
   cp .env.example .env
   ```

2. Setup database (migrations + seeders)

   ```bash
   make db-setup
   ```

3. Start development server with hot reload

   ```bash
   make dev
   ```

### Make Commands

```bash
make help          # Show all available commands
make dev           # Development with hot reload
make build         # Build production binary
make db-setup      # Setup database
```

📖 **Full guide**: [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)

## API Documentation

### Available Endpoints

#### Authentication

- `POST /api/v1/auth/signin` - User signin

### Testing API

**Health Check:**

```bash
curl http://localhost:8080/health
```

**Signin:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "login": "admin@tuai.com",
    "password": "admin123"
  }'
```

## Logging

Project ini menggunakan **Uber's Zap** dengan **Ginzap** middleware untuk logging.

- **Development**: Colored console output, Debug level
- **Production**: JSON output, Info level

## Documentation

- [Development Guide](docs/DEVELOPMENT.md) - Hot reload, Make commands, debugging
- [Architecture Guide](docs/ARCHITECTURE.md) - Project structure & patterns
- [API Signin](docs/API_SIGNIN.md) - Signin endpoint documentation
