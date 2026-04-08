#!/bin/bash
set -e

MIGRATIONS_DIR="$(dirname "$0")/../migrations"

echo "Running migrations..."
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/001_init.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/002_users_homes.up.sql"
echo "Migrations complete."
