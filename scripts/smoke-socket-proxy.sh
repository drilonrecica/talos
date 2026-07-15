#!/bin/bash
set -euo pipefail

IMAGE="ghcr.io/wollomatic/socket-proxy:1.12.3@sha256:9e781fbe79315355d08901832f639119aa332ac27ee6157fc7f2fab5193c8600"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
  echo "Docker is unavailable; skipping socket proxy smoke test."
  exit 0
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is unavailable; skipping socket proxy smoke test."
  exit 0
fi

directory="$(mktemp -d)"
container="binnacle-socket-proxy-smoke-$$"
docker_gid="$(stat -c '%g' /var/run/docker.sock)"
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$directory"
}
trap cleanup EXIT

docker run -d --name "$container" --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  --user "0:$docker_gid" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v "$directory:/var/run/binnacle-docker" \
  "$IMAGE" \
  -loglevel=warn \
  -allowHEAD=/_ping \
  '-allowGET=/v1\.[0-9]+/version' \
  '-allowGET=/v1\.[0-9]+/containers/json' \
  '-allowGET=/v1\.[0-9]+/containers/[^/]+/(json|stats|logs)' \
  '-allowGET=/v1\.[0-9]+/events' \
  -proxysocketendpoint=/var/run/binnacle-docker/docker.sock \
  -proxysocketendpointfilemode=0660 >/dev/null

socket="$directory/docker.sock"
for _ in $(seq 1 30); do
  test -S "$socket" && break
  sleep 0.2
done
test -S "$socket"

status() {
  curl --silent --show-error --unix-socket "$socket" --output /dev/null --write-out '%{http_code}' "$@"
}

test "$(status --head http://localhost/_ping)" = "200"
test "$(status http://localhost/v1.55/version)" != "403"
test "$(status --request POST http://localhost/v1.55/containers/create)" = "403"
test "$(status http://localhost/v1.55/containers/example/archive)" = "403"
test "$(status http://localhost/v1.55/images/json)" = "403"

echo "Socket proxy allowlist smoke test passed."
