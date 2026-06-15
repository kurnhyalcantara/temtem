#!/usr/bin/env bash
# Run database migrations without a local `migrate` binary, using the
# migrate/migrate Docker image against the compose Postgres.
#
# Usage: scripts/migrate.sh [up|down 1|version|...]
set -euo pipefail

cd "$(dirname "$0")/.."

docker compose -f deployments/docker-compose.yml run --rm migrate \
  -path=/migrations \
  -database="postgres://temtem:temtem@postgres:5432/temtem?sslmode=disable" \
  "${@:-up}"
