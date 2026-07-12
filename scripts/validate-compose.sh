#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/packaging/docker/docker-compose.yml"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  BINNACLE_SETUP_TOKEN=dummy docker compose -f "$COMPOSE_FILE" config >/dev/null
  echo "Compose file is valid."
else
  echo "docker compose not available; skipping live validation."
fi
