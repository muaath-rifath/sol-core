#!/bin/bash
set -e

MIGRATIONS_DIR="$(dirname "$0")/../migrations"

echo "Running migrations..."
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/001_init.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/002_users_homes.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/003_invite_improvements.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/004_email_case_fix.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/005_home_name_unique.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/006_home_cascade_delete.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/007_firmware_versions.up.sql"
echo "Migrations complete."
