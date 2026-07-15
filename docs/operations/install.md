# Installation

Binnacle is distributed as a container image. The supported paths are Coolify
(one-click service), Docker Compose, or GHCR.

> **Current release:** For production, pin the immutable `v0.6.0` image tag.
> The `stable` tag tracks the latest stable release. Qualify a source-built
> `local` image before using it in production.

> **Qualification gates:** advanced authentication and portability are
> implemented but disabled in packaged defaults. Leave both feature flags off
> unless you are explicitly running their acceptance tests.

## Requirements

- Linux host (x86_64 or arm64)
- Docker Engine 29.5.1 or newer with a reachable Unix socket
- Read access to `/proc`, `/sys`, `/etc/passwd`, and `/etc/os-release` on the host
- A persistent volume mounted at `/var/lib/binnacle`

## Generate a setup token

Binnacle checks the daemon release during production startup and fails closed
if the version is missing, malformed, or older than 29.5.1. Upgrade the host
Engine before deploying Binnacle. A distribution security backport reported
under an older release string does not satisfy this requirement. Demo mode
does not connect to Docker and has no Engine requirement.

On first start, Binnacle requires a high-entropy setup token. Generate one before deploying:

```bash
export BINNACLE_SETUP_TOKEN="$(openssl rand -hex 32)"
```

Store it in a password manager. After the first administrator is created, the setup token is permanently disabled and cannot be reused.

## Install with Coolify

1. Add the Binnacle service template to your Coolify instance (or use **Docker Compose Empty** and paste `packaging/coolify/binnacle.yaml`).
2. In **General**, change the generated **Service Name** (`service-...`) to `binnacle`.
3. Deploy the stack. Coolify generates a persistent 64-character setup token automatically.
4. Expose the service on your chosen domain. Coolify's proxy routes to container port `8080`.
5. Copy `SERVICE_HEX_64_BINNACLE` from the stack environment, open the URL, and complete onboarding.

The Coolify template mounts the host `/proc`, `/sys`, `/etc/passwd`,
`/etc/os-release`, and the Docker socket. It runs `read_only: true` with
`no-new-privileges` and a 128 MiB memory limit.

The template intentionally exposes only first-run setup and optional Coolify
enrichment variables. Binnacle's secure defaults apply to advanced
authentication, portability, private-network access, Prometheus, and
notification limits. Edit the Compose file only when deliberately enabling one
of those features.

Coolify intentionally prefixes Docker containers, networks, and volumes with
the stack UUID. Keep those generated names: they prevent collisions between
multiple Binnacle deployments. The Compose service and volume names themselves
are already stable (`binnacle`, `docker-socket-proxy`, `binnacle-data`, and
`binnacle-docker-api`).

## Install with Docker Compose

For a published release:

```bash
git clone https://github.com/drilonrecica/binnacle.git
cd binnacle
export BINNACLE_SETUP_TOKEN="$(openssl rand -hex 32)"
docker compose -f packaging/docker/docker-compose.yml up -d
```

Then open `http://127.0.0.1:8080` and complete onboarding.

To evaluate the unreleased source tree, build the local image and explicitly
override the Compose image:

```bash
make image
export BINNACLE_IMAGE=ghcr.io/drilonrecica/binnacle:local
docker compose -f packaging/docker/docker-compose.yml up -d
```

This uses the same mounts, limits, and persistent volume as the published
release path. Do not treat an unqualified development image as a release.

## Bootstrap credentials

Instead of using the setup token, you can bootstrap the admin account from an environment variable or Docker secret:

```yaml
environment:
  BINNACLE_ADMIN_USERNAME: admin
  BINNACLE_ADMIN_PASSWORD_FILE: /run/secrets/binnacle_admin_password
```

When bootstrap credentials are present and no user exists, Binnacle creates the account automatically. The setup token is still required for public/non-loopback first runs unless you are using local development mode.

## Docker socket security

The Docker Unix socket is not logically read-only. Mounting it read-only (`:ro`)
does not prevent container mutation through the Engine API. The production
Compose and Coolify manifests therefore mount the daemon socket only into a
pinned socket-proxy sidecar. Binnacle receives a separate Unix socket that
allows only ping, version, container list/inspect/stats/logs, and events reads.
Do not replace this with a direct daemon-socket mount in production. A direct
mount remains possible only as an explicit legacy deployment override.

For Docker secrets, mount the master key as a file rather than exposing it in
the container environment:

```yaml
services:
  binnacle:
    environment:
      BINNACLE_MASTER_KEY_FILE: /run/secrets/binnacle_master_key
    secrets:
      - binnacle_master_key
secrets:
  binnacle_master_key:
    file: ./secrets/binnacle-master-key
```

The file must contain a raw or encoded 32-byte key and should be readable only
by the Binnacle container. `BINNACLE_MASTER_KEY` remains supported for upgrades,
but do not configure it together with `BINNACLE_MASTER_KEY_FILE`.

## Environment variables

Key variables you may need to set at deployment time:

- `BINNACLE_SETUP_TOKEN` — required for first install.
- `BINNACLE_DATA_DIR` — defaults to `/var/lib/binnacle`.
- `BINNACLE_HOST_PROC` — defaults to `/host/proc`.
- `BINNACLE_HOST_SYS` — defaults to `/host/sys`.
- `BINNACLE_HOST_PASSWD` — defaults to `/host/etc/passwd` for process username resolution.
- `BINNACLE_LOGS_MAX_LINES` — bounded log response ceiling; defaults to `5000`.
- `BINNACLE_LOGS_MAX_RESPONSE_BYTES` — bounded log byte ceiling; defaults to `1048576`.
- `BINNACLE_LOGS_REDACTION_PATTERNS` — up to 16 additional RE2 patterns separated by `||`.
- `BINNACLE_DOCKER_SOCKET` — the packaged default is the filtered socket at `/var/run/binnacle-docker/docker.sock`; standalone binaries default to `/var/run/docker.sock`.
- `BINNACLE_MASTER_KEY` — raw/base64 32-byte key or 64-character hex key for notification secrets.
- `BINNACLE_MASTER_KEY_FILE` — absolute path to a file containing the master key; preferred for production secrets.
- `BINNACLE_COOLIFY_URL` and `BINNACLE_COOLIFY_API_TOKEN[_FILE]` — optional read-only Coolify enrichment.
- `BINNACLE_FEATURE_ADVANCED_AUTH` — enables TOTP and trusted-proxy authentication; defaults to `false` pending qualification.
- `BINNACLE_FEATURE_PORTABILITY` — enables personal API tokens, exports, and eligibility for Prometheus; defaults to `false` pending qualification.
- `BINNACLE_PROMETHEUS_ENABLED` — enables token-authenticated `/metrics` only when portability is also enabled; defaults to `false`.
- `BINNACLE_AUTH_MODE` — `local`, `proxy`, or `local_and_proxy`; defaults to `local`. Proxy modes require advanced authentication to be enabled.
- `BINNACLE_AUTH_PROXY_CIDRS`, `BINNACLE_AUTH_IDENTITY_HEADER`, and `BINNACLE_AUTH_ALLOWED_SUBJECT` — required together for proxy modes; CIDRs must be exact `/32` or `/128` immediate peers.
- `BINNACLE_TRUSTED_PROXY_CIDRS` — exact `/32` or `/128` immediate peers allowed to supply forwarding headers.
- `BINNACLE_NOTIFICATIONS_ALLOW_PRIVATE_TARGETS` — private webhook/SMTP opt-in; defaults to `false` and requires restart.
- `BINNACLE_NOTIFICATIONS_MAX_CONCURRENCY` — delivery workers; defaults to `4`.
- `BINNACLE_NOTIFICATIONS_QUEUE_CAPACITY` — dispatch queue; defaults to `1000`.
- `BINNACLE_NOTIFICATIONS_DELIVERY_TIMEOUT` — per-attempt timeout; defaults to `15s`.
- `BINNACLE_NOTIFICATIONS_REMINDER_INTERVAL` — open-incident reminders; defaults to `2h`.

Additional settings are available in the authenticated Settings interface. See
the [product boundaries](../PRODUCT.md) for the supported runtime and security
model.
