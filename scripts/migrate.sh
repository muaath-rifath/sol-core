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
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/008_activity_logs.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/009_split_appliances.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/010_firmware_builds.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/011_ota_attempts.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/012_ota_attempts_idempotency.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/013_member_permissions.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/014_member_room_capabilities.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/015_fix_reserved_gpio_pins.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/016_appliance_embeddings.up.sql"
docker exec -i sol-postgres psql -U sol -d sol < "$MIGRATIONS_DIR/017_firmware_model_key.up.sql"
echo "Migrations complete."
