#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-only
# Release gate automation for Binnacle.
# Produces a Markdown release record in RELEASE_RECORD_DIR.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RELEASE_RECORD_DIR="${RELEASE_RECORD_DIR:-release-record}"
mkdir -p "$RELEASE_RECORD_DIR"

VERSION="${VERSION:-v0.4.0}"
COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD)"
SHORT_COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
HOST_OS="$(uname -s)"
HOST_ARCH="$(uname -m)"
GO_VERSION="$(go version | awk '{print $3}')"

RECORD="$RELEASE_RECORD_DIR/$VERSION-$SHORT_COMMIT.md"
rm -f "$RECORD"

touch "$RECORD"
results=()

record() {
  printf '%s\n' "$*" >> "$RECORD"
}

pass() {
  results+=("$1|PASS|$2")
}

fail() {
  results+=("$1|FAIL|$2")
}

skip() {
  results+=("$1|SKIP|$2")
}

run_check() {
  local name="$1"
  shift
  if (cd "$ROOT_DIR" && "$@" >> "$RELEASE_RECORD_DIR/build.log" 2>&1); then
    pass "$name" "$(printf '%q ' "$@")"
  else
    fail "$name" "$(printf '%q ' "$@") (see build.log)"
  fi
}

run_optional() {
  local name="$1"
  shift
  if (cd "$ROOT_DIR" && "$@" >> "$RELEASE_RECORD_DIR/build.log" 2>&1); then
    pass "$name" "$(printf '%q ' "$@")"
  else
    skip "$name" "$(printf '%q ' "$@") failed or unavailable"
  fi
}

record "# Binnacle $VERSION release record"
record ""
record "- **Commit:** $COMMIT"
record "- **Date:** $DATE"
record "- **Host:** $HOST_OS ($HOST_ARCH)"
record "- **Go:** $GO_VERSION"
record ""

# Determine container tool.
DOCKER_CMD="${DOCKER:-}"
if [[ -z "$DOCKER_CMD" ]]; then
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    DOCKER_CMD=docker
  elif command -v podman >/dev/null 2>&1; then
    DOCKER_CMD=podman
  fi
fi
record "- **Container tool:** ${DOCKER_CMD:-none}"
record ""

# Clean previous build log.
rm -f "$RELEASE_RECORD_DIR/build.log"

record "## Gates"
record ""

# 1. Local CI-quality check.
run_check "make check" make check

# Checks and alerts security/lifecycle qualification.
run_check "security and diagnostic race tests" go test -race ./internal/checks ./internal/alerts ./internal/notifications ./internal/outbound ./internal/api ./internal/diagnostics ./internal/dockerapi ./internal/demo

# 2. License and security policy presence.
if [[ -s "$ROOT_DIR/LICENSE" && -s "$ROOT_DIR/SECURITY.md" ]]; then
  pass "license and security policy" "LICENSE and SECURITY.md present"
else
  fail "license and security policy" "LICENSE or SECURITY.md missing/empty"
fi

# 3. Build binary.
run_check "build binary" make build VERSION="$VERSION"
run_check "binary version" bash -c 'test "$(./bin/binnacle --version)" = "$1"' _ "$VERSION"

# Packaging templates must be valid before an image can be published.
run_check "Compose validation" scripts/validate-compose.sh
run_check "Coolify validation" scripts/validate-coolify-template.sh

# 4. Build container image.
if [[ -n "$DOCKER_CMD" ]]; then
  run_check "build container image" make image DOCKER="$DOCKER_CMD" VERSION="$VERSION"
  run_check "container version" "$DOCKER_CMD" run --rm --entrypoint /usr/local/bin/binnacle ghcr.io/drilonrecica/binnacle:local --version
else
  skip "build container image" "no container runtime available"
fi

# 5. Demo container smoke test.
if [[ -n "$DOCKER_CMD" ]]; then
  demo_port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
  container_name="binnacle-release-gate-$demo_port"
  if "$DOCKER_CMD" run -d --name "$container_name" -e BINNACLE_SETUP_TOKEN=binnacle-release-gate-token-32chars -p "127.0.0.1:$demo_port:8080" "ghcr.io/drilonrecica/binnacle:local" --demo --demo-seed 1 >> "$RELEASE_RECORD_DIR/build.log" 2>&1; then
    for _ in {1..30}; do
      if curl -sf "http://127.0.0.1:$demo_port/healthz" >/dev/null 2>&1; then
        break
      fi
      sleep 1
    done
    if curl -sf "http://127.0.0.1:$demo_port/healthz" >/dev/null 2>&1; then
      pass "demo container smoke" "container responded on port $demo_port"
    else
      fail "demo container smoke" "container did not respond on port $demo_port"
    fi
    "$DOCKER_CMD" stop -t 5 "$container_name" >> "$RELEASE_RECORD_DIR/build.log" 2>&1 || true
    "$DOCKER_CMD" rm "$container_name" >> "$RELEASE_RECORD_DIR/build.log" 2>&1 || true
  else
    fail "demo container smoke" "container failed to start"
  fi
else
  skip "demo container smoke" "no container runtime available"
fi

# 6. Benchmark.
if [[ -f "$ROOT_DIR/scripts/benchmark.py" ]]; then
  run_check "benchmark" make benchmark
else
  skip "benchmark" "benchmark script missing"
fi

# 7. Full browser qualification.
if command -v pnpm >/dev/null 2>&1 && [[ -d "$ROOT_DIR/web/tests/e2e" ]]; then
  e2e_data="$(mktemp -d)"
  export BINNACLE_DATA_DIR="$e2e_data"
  export BINNACLE_RUNTIME_DIR="$e2e_data/runtime"
  export BINNACLE_SETUP_TOKEN=binnacle-e2e-smoke-token-32chars-long
  "$ROOT_DIR/bin/binnacle" --demo --demo-seed 1 >> "$RELEASE_RECORD_DIR/build.log" 2>&1 &
  e2e_pid=$!
  for _ in {1..30}; do
    if curl -sf "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  if curl -sf "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; then
    # Serialize the broad suite on release hosts. Parallel Chromium instances
    # can starve page navigation while the demo server and container runtime
    # are active, producing non-deterministic timeout failures.
    run_check "e2e application" pnpm --dir web exec playwright test --workers=1
    run_check "e2e landing" pnpm --dir web test:landing
    run_check "e2e visual regression" pnpm --dir web test:e2e:visual
  else
    fail "e2e browser qualification" "demo server did not start"
  fi
  kill "$e2e_pid" >> "$RELEASE_RECORD_DIR/build.log" 2>&1 || true
  wait "$e2e_pid" >> "$RELEASE_RECORD_DIR/build.log" 2>&1 || true
  rm -rf "$e2e_data"
else
  skip "e2e smoke" "pnpm or e2e tests unavailable"
fi

# 8. Supply-chain scan (optional; requires network and tooling).
run_optional "supply-chain scan" make vuln

# Emit summary table.
record "| Gate | Result | Evidence |"
record "| --- | --- | --- |"
for entry in "${results[@]}"; do
  IFS='|' read -r gate result evidence <<< "$entry"
  record "| $gate | $result | $evidence |"
done
record ""

# Append benchmark summary if available.
if [[ -f "$ROOT_DIR/benchmark-report.json" ]]; then
  record "## Benchmark summary"
  record ""
  record '```json'
  cat "$ROOT_DIR/benchmark-report.json" >> "$RECORD"
  record ""
  record '```'
  record ""
fi

record "## Decision"
record ""
if grep -q '| FAIL |' "$RECORD"; then
  record "**NO-GO:** at least one required gate failed."
  echo "Release gate FAILED. Record: $RECORD"
  exit 1
else
  record "**GO:** all required gates passed; optional gates may be skipped."
  echo "Release gate PASSED. Record: $RECORD"
fi
