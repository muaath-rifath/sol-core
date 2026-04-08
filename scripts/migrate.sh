#!/bin/bash
set -e

echo "Running migrations..."
docker exec -i sol-postgres psql -U sol -d sol < "$(dirname "$0")/../migrations/001_init.up.sql"
echo "Migrations complete."
