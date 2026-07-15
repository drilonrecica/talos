#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/packaging/docker/docker-compose.yml"
TEMPLATE_FILE="$ROOT_DIR/packaging/coolify/binnacle.yaml"
SOURCE_FILE="$ROOT_DIR/compose.coolify.yml"

python3 - "$COMPOSE_FILE" "$TEMPLATE_FILE" "$SOURCE_FILE" <<'PY'
import sys, yaml

compose = yaml.safe_load(open(sys.argv[1]))
template = yaml.safe_load(open(sys.argv[2]))
source = yaml.safe_load(open(sys.argv[3]))

def service(doc, name="binnacle"):
    return doc["services"][name]

c, t = service(compose), service(template)
cp, tp, sp = service(compose, "docker-socket-proxy"), service(template, "docker-socket-proxy"), service(source, "docker-socket-proxy")
compose_image = c.get("image")
if compose_image == "${BINNACLE_IMAGE:-ghcr.io/drilonrecica/binnacle:stable}":
    compose_image = "ghcr.io/drilonrecica/binnacle:stable"

checks = [
    ("image", compose_image, t.get("image")),
    ("read_only", c.get("read_only"), t.get("read_only")),
    ("privileged", c.get("privileged"), t.get("privileged")),
    ("user", c.get("user"), t.get("user")),
    ("labels", c.get("labels", {}), t.get("labels", {})),
    ("security_opt", sorted(c.get("security_opt", [])), sorted(t.get("security_opt", []))),
    ("environment keys", sorted(c.get("environment", {}).keys()), sorted(t.get("environment", {}).keys())),
    ("volume mounts", sorted(c.get("volumes", [])), sorted(t.get("volumes", []))),
    ("restart", c.get("restart"), t.get("restart")),
    ("healthcheck", c.get("healthcheck"), t.get("healthcheck")),
    ("resource configuration", c.get("deploy", {}).get("resources", {}),
                               t.get("deploy", {}).get("resources", {})),
    ("socket proxy", cp, tp),
]

failed = False
for name, expected, actual in checks:
    if expected != actual:
        print(f"DRIFT: {name} differs")
        print(f"  compose:  {expected}")
        print(f"  template: {actual}")
        failed = True

if failed:
    sys.exit(1)

print("Coolify template matches canonical Compose deployment.")

s = service(source)
required = {
    "build.context": s.get("build", {}).get("context") == ".",
    "build.dockerfile": s.get("build", {}).get("dockerfile") == "packaging/docker/Dockerfile",
    "read_only": s.get("read_only") is True,
    "privileged": s.get("privileged") is False,
    "healthcheck": s.get("healthcheck", {}).get("test") == ["CMD", "/usr/local/bin/binnacle", "--healthcheck"],
}
missing = [name for name, valid in required.items() if not valid]
if missing:
    raise SystemExit("Invalid source-build Coolify configuration: " + ", ".join(missing))

for name, expected, actual in (
    ("labels", c.get("labels", {}), s.get("labels", {})),
    ("restart", c.get("restart"), s.get("restart")),
    ("healthcheck", c.get("healthcheck"), s.get("healthcheck")),
    ("resource configuration", c.get("deploy", {}).get("resources", {}), s.get("deploy", {}).get("resources", {})),
    ("environment keys", sorted(c.get("environment", {}).keys()), sorted(s.get("environment", {}).keys())),
    ("volume mounts", sorted(c.get("volumes", [])), sorted(s.get("volumes", []))),
    ("depends_on", c.get("depends_on", {}), s.get("depends_on", {})),
    ("socket proxy", cp, sp),
):
    if expected != actual:
        raise SystemExit(f"Source-build Coolify drift: {name} differs\n  compose: {expected}\n  source:  {actual}")

for key in ("BINNACLE_DOCKER_SOCKET", "BINNACLE_CHECKS_ALLOW_PRIVATE_TARGETS", "BINNACLE_MASTER_KEY", "BINNACLE_MASTER_KEY_FILE",
            "BINNACLE_NOTIFICATIONS_ALLOW_PRIVATE_TARGETS", "BINNACLE_NOTIFICATIONS_MAX_CONCURRENCY",
            "BINNACLE_NOTIFICATIONS_QUEUE_CAPACITY", "BINNACLE_NOTIFICATIONS_DELIVERY_TIMEOUT",
            "BINNACLE_NOTIFICATIONS_REMINDER_INTERVAL"):
    if key not in s.get("environment", {}):
        raise SystemExit(f"Source-build Coolify configuration does not pass through {key}")

raw_socket = "/var/run/docker.sock:/var/run/docker.sock:ro"
for label, doc in (("Compose", compose), ("Coolify template", template), ("source-build Coolify", source)):
    app_mounts = service(doc).get("volumes", [])
    proxy_mounts = service(doc, "docker-socket-proxy").get("volumes", [])
    if any("/var/run/docker.sock" in mount for mount in app_mounts):
        raise SystemExit(f"{label} exposes the raw Docker socket to Binnacle")
    if raw_socket not in proxy_mounts:
        raise SystemExit(f"{label} socket proxy does not have the read-only daemon socket mount")
print("Source-build Coolify Compose is valid.")
PY
