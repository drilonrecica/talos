# Installation

Binnacle is distributed as a container image. The supported paths are Coolify
(one-click service), Docker Compose, or GHCR.

> **Development status:** v0.4 is implemented but no v0.4 tag or image has been
> published. The `stable` examples below apply to published releases; qualify a
> source-built `local` image before using unreleased code in production.

## Requirements

- Linux host (x86_64 or arm64)
- Docker Engine with a reachable Unix socket
- Read access to `/proc`, `/sys`, `/etc/passwd`, and `/etc/os-release` on the host
- A persistent volume mounted at `/var/lib/binnacle`

## Generate a setup token

On first start, Binnacle requires a high-entropy setup token. Generate one before deploying:

```bash
export BINNACLE_SETUP_TOKEN="$(openssl rand -hex 32)"
```

Store it in a password manager. After the first administrator is created, the setup token is permanently disabled and cannot be reused.

## Install with Coolify

1. Add the Binnacle service template to your Coolify instance (or use **Docker Compose Empty** and paste `packaging/coolify/binnacle.yaml`).
2. Set `BINNACLE_SETUP_TOKEN` in the Coolify environment variables.
3. Expose the service on your chosen domain. Coolify's proxy routes to container port `8080`.
4. Open the URL and complete onboarding.

The Coolify template mounts the host `/proc`, `/sys`, `/etc/passwd`,
`/etc/os-release`, and the Docker socket. It runs `read_only: true` with
`no-new-privileges` and a 128 MiB memory limit.

## Install with Docker Compose

For a published release:

```bash
git clone https://github.com/drilonrecica/binnacle.git
cd binnacle
export BINNACLE_SETUP_TOKEN="$(openssl rand -hex 32)"
export DOCKER_GID="$(getent group docker | cut -d: -f3)"
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

The Docker Unix socket is not logically read-only. Mounting it read-only (`:ro`) does not prevent container mutation through the Engine API. Binnacle contains no Docker mutation code paths and does not proxy the Docker API, but the socket still grants broad privileges. For hardened deployments, run a restricted read-only Docker socket proxy and set `BINNACLE_DOCKER_SOCKET` to its address.

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
- `BINNACLE_DOCKER_SOCKET` — defaults to `/var/run/docker.sock`.
- `BINNACLE_MASTER_KEY` — raw/base64 32-byte key or 64-character hex key for notification secrets.
- `BINNACLE_NOTIFICATIONS_ALLOW_PRIVATE_TARGETS` — private webhook/SMTP opt-in; defaults to `false` and requires restart.
- `BINNACLE_NOTIFICATIONS_MAX_CONCURRENCY` — delivery workers; defaults to `4`.
- `BINNACLE_NOTIFICATIONS_QUEUE_CAPACITY` — dispatch queue; defaults to `1000`.
- `BINNACLE_NOTIFICATIONS_DELIVERY_TIMEOUT` — per-attempt timeout; defaults to `15s`.
- `BINNACLE_NOTIFICATIONS_REMINDER_INTERVAL` — open-incident reminders; defaults to `2h`.

Additional settings are available in the authenticated Settings interface. See
the [product boundaries](../PRODUCT.md) for the supported runtime and security
model.
