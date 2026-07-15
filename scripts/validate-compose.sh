#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/packaging/docker/docker-compose.yml"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  DOCKER_GID="${DOCKER_GID:-$(stat -c '%g' /var/run/docker.sock 2>/dev/null || id -g)}"
  export DOCKER_GID
  rendered="$(BINNACLE_SETUP_TOKEN=dummy docker compose -f "$COMPOSE_FILE" config)"
  grep -F 'image: ghcr.io/drilonrecica/binnacle:stable' <<<"$rendered" >/dev/null
  grep -F 'ghcr.io/wollomatic/socket-proxy:1.12.3@sha256:9e781fbe79315355d08901832f639119aa332ac27ee6157fc7f2fab5193c8600' <<<"$rendered" >/dev/null
  binnacle_service="$(awk '/^  binnacle:/{inside=1; next} inside && /^  [a-zA-Z0-9_-]+:/{exit} inside{print}' <<<"$rendered")"
  if grep -F '/var/run/docker.sock' <<<"$binnacle_service" >/dev/null; then
    echo "Binnacle must not mount the raw Docker socket." >&2
    exit 1
  fi
  echo "Compose file is valid."
else
  echo "docker compose not available; skipping live validation."
fi
