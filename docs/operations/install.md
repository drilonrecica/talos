# Installation

Binnacle is distributed as a container image. The supported paths are Coolify (one-click service), Docker Compose, or GHCR.

## Requirements

- Linux host (x86_64 or arm64)
- Docker Engine with a reachable Unix socket
- Read access to `/proc`, `/sys`, and `/etc/os-release` on the host
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

The Coolify template mounts the host `/proc`, `/sys`, `/etc/os-release`, and the Docker socket. It runs `read_only: true` with `no-new-privileges` and a 128 MiB memory limit.

## Install with Docker Compose

```bash
git clone https://github.com/drilonrecica/binnacle.git
cd binnacle
export BINNACLE_SETUP_TOKEN="$(openssl rand -hex 32)"
docker compose -f packaging/docker/docker-compose.yml up -d
```

Then open `http://127.0.0.1:8080` and complete onboarding.

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
- `BINNACLE_DOCKER_SOCKET` — defaults to `/var/run/docker.sock`.
- `BINNACLE_MASTER_KEY` — 32-byte hex key for encrypting UI-entered secrets.

See `docs/SPEC.md` for the full configuration model.
