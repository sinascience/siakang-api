#!/bin/bash
set -e

# Function to check if database is ready
wait_for_db() {
    echo "Waiting for database to be ready..."

    # Parse DB_HOST and DB_PORT from environment
    DB_HOST=${DB_HOST:-localhost}
    DB_PORT=${DB_PORT:-5432}

    echo "Attempting to connect to:"
    echo "  Host: $DB_HOST"
    echo "  Port: $DB_PORT"
    echo "  User: $DB_USER"
    echo "  Database: $DB_NAME"
    echo ""

    # Maximum wait time in seconds (5 minutes)
    MAX_WAIT=300
    ELAPSED=0

    until PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c '\q' 2>/dev/null; do
        if [ $ELAPSED -ge $MAX_WAIT ]; then
            echo "ERROR: Database is not ready after ${MAX_WAIT} seconds. Exiting..."
            exit 1
        fi

        echo "Database is unavailable - sleeping (${ELAPSED}s/${MAX_WAIT}s)"
        sleep 2
        ELAPSED=$((ELAPSED + 2))
    done

    echo "Database is ready!"
}

# Function to run migrations
run_migrations() {
    echo "Running database migrations..."

    # Construct database URL
    DB_URL="postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE:-disable}"

    # Run core migrations
    if [ -d "/app/internal/database/migrations/core" ]; then
        echo "Running core migrations..."
        migrate -path /app/internal/database/migrations/core \
                -database "${DB_URL}&x-migrations-table=schema_migrations_core" \
                up || {
                    echo "ERROR: Core migrations failed"
                    exit 1
                }
        echo "Core migrations completed successfully!"
    fi

    # Run CRM migrations
    if [ -d "/app/internal/database/migrations/crm" ]; then
        echo "Running CRM migrations..."
        migrate -path /app/internal/database/migrations/crm \
                -database "${DB_URL}&x-migrations-table=schema_migrations_crm" \
                up || {
                    echo "ERROR: CRM migrations failed"
                    exit 1
                }
        echo "CRM migrations completed successfully!"
    fi

    # Run Finance migrations
    if [ -d "/app/internal/database/migrations/finance" ]; then
        echo "Running Finance migrations..."
        migrate -path /app/internal/database/migrations/finance \
                -database "${DB_URL}&x-migrations-table=schema_migrations_finance" \
                up || {
                    echo "ERROR: Finance migrations failed"
                    exit 1
                }
        echo "Finance migrations completed successfully!"
    fi

    echo "All migrations completed successfully!"
}

# Main execution
echo "==================================="
echo "Tuai Backend - Starting up"
echo "==================================="
echo ""
echo "Database Configuration:"
echo "  Host: ${DB_HOST:-localhost}"
echo "  Port: ${DB_PORT:-5432}"
echo "  User: ${DB_USER}"
echo "  Database: ${DB_NAME}"
echo "  SSL Mode: ${DB_SSLMODE:-disable}"
echo ""

# Check if SKIP_MIGRATION is set
if [ "${SKIP_MIGRATION}" = "true" ]; then
    echo "SKIP_MIGRATION is set to true. Skipping migrations..."
else
    # Wait for database to be ready
    wait_for_db

    # Run migrations
    run_migrations
fi

echo "==================================="
echo "Starting application..."
echo "==================================="

# Execute the main command (passed as arguments to this script)
exec "$@"
