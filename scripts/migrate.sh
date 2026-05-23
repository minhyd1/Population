#!/bin/bash
# scripts/migrate.sh - Chạy database migrations
# Usage: ./scripts/migrate.sh

set -e

source configs/.env 2>/dev/null || true

DB_URL="postgresql://${DB_USER:-postgres}:${DB_PASSWORD}@${DB_HOST:-localhost}:${DB_PORT:-5432}/${DB_NAME:-population_db}?sslmode=${DB_SSLMODE:-disable}"

echo "🔄 Running migrations..."

for file in scripts/*.sql; do
    echo "  → Applying $file"
    psql "$DB_URL" -f "$file"
done

echo "✅ Migrations complete!"
echo ""
echo "🌱 Running seeder..."
ENCRYPTION_KEY="$ENCRYPTION_KEY" \
DB_HOST="${DB_HOST:-localhost}" \
DB_USER="${DB_USER:-postgres}" \
DB_PASSWORD="$DB_PASSWORD" \
DB_NAME="${DB_NAME:-population_db}" \
go run scripts/seed.go

echo "🚀 Ready! Run: go run ./cmd/server/..."
