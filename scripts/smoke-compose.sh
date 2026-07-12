#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/packaging/docker/docker-compose.yml"
SETUP_TOKEN="${BINNACLE_SETUP_TOKEN:-$(openssl rand -hex 32)}"
export BINNACLE_SETUP_TOKEN="$SETUP_TOKEN"

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  echo "docker compose not available; skipping smoke test."
  exit 0
fi

cleanup() {
  docker compose -f "$COMPOSE_FILE" down -v || true
}
trap cleanup EXIT

docker compose -f "$COMPOSE_FILE" up -d --wait

for i in $(seq 1 30); do
  if curl -fsS -o /dev/null http://127.0.0.1:8080/api/v1/session; then
    echo "Smoke test passed: Binnacle is reachable."
    exit 0
  fi
  sleep 1
done

echo "Smoke test failed: Binnacle did not become reachable."
exit 1
