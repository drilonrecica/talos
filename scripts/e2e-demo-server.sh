#!/bin/bash
set -euo pipefail

# Build and run a deterministic demo Binnacle server for end-to-end tests.
# The data directory is temporary and removed on exit.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DATA_DIR="$(mktemp -d)"
export BINNACLE_DATA_DIR="$DATA_DIR"
export BINNACLE_RUNTIME_DIR="$DATA_DIR/runtime"

cleanup() {
  rm -rf "$DATA_DIR"
}
trap cleanup EXIT

if ! test -x "$ROOT_DIR/bin/binnacle"; then
  echo "bin/binnacle not found; run 'make build' first." >&2
  exit 1
fi
exec "$ROOT_DIR/bin/binnacle" --demo --demo-seed 1
