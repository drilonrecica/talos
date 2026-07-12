#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/packaging/docker/docker-compose.yml"
TEMPLATE_FILE="$ROOT_DIR/packaging/coolify/binnacle.yaml"

python3 - "$COMPOSE_FILE" "$TEMPLATE_FILE" <<'PY'
import sys, yaml

compose = yaml.safe_load(open(sys.argv[1]))
template = yaml.safe_load(open(sys.argv[2]))

def service(doc):
    return doc["services"]["binnacle"]

c, t = service(compose), service(template)

checks = [
    ("image", c.get("image"), t.get("image")),
    ("read_only", c.get("read_only"), t.get("read_only")),
    ("privileged", c.get("privileged"), t.get("privileged")),
    ("user", c.get("user"), t.get("user")),
    ("security_opt", sorted(c.get("security_opt", [])), sorted(t.get("security_opt", []))),
    ("environment keys", sorted(c.get("environment", {}).keys()), sorted(t.get("environment", {}).keys())),
    ("volume mounts", sorted(c.get("volumes", [])), sorted(t.get("volumes", []))),
    ("memory limit", c.get("deploy", {}).get("resources", {}).get("limits", {}).get("memory"),
                    t.get("deploy", {}).get("resources", {}).get("limits", {}).get("memory")),
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
PY
