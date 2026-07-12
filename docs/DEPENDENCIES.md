# Dependency inventory

This document lists the key frameworks, tools, libraries, and base images used by Binnacle, the versions we target, and the policy for keeping them current.

Last updated: 2026-07-12

## Policy

- **Runtime / build tools:** bump to the latest stable major/minor release during each pre-release pass, or when a security patch is available.
- **Language toolchains:** stay on the latest stable Go and Node.js LTS releases.
- **Action pins:** GitHub Actions are pinned to the latest stable semantic-version tag. Floating tags such as `@master` or `@v0` are avoided in CI.
- **Exceptions:** if the latest stable major of an ecosystem package is not yet supported by its peer ecosystem (e.g. TypeScript 7 is not supported by `typescript-eslint` 8), we pin to the latest compatible stable version and document the reason.

## Go

| Component | Version | Notes |
|-----------|---------|-------|
| Go toolchain | `1.26.x` (latest patch `1.26.5`) | Set in `go.mod`, CI, and Dockerfile. |
| Direct dependencies | see `go.mod` | All direct dependencies are on their latest stable releases. |

Key direct dependencies:

- `github.com/BurntSushi/toml` `v1.6.0`
- `github.com/docker/docker` `v28.5.2+incompatible`
- `github.com/mattn/go-sqlite3` `v1.14.47`
- `golang.org/x/crypto` `v0.54.0`
- `golang.org/x/sys` `v0.47.0`

## Frontend

| Component | Version | Notes |
|-----------|---------|-------|
| Node.js | `24.x` LTS (latest patch `24.18.0`) | Specified in root `package.json` engines and Dockerfile. |
| pnpm | `11.12.0` | Specified via `packageManager` in root `package.json`. |
| Svelte | `5.56.4` | Latest stable. |
| Vite | `8.1.4` | Latest stable. |
| ESLint | `10.7.0` | Latest stable. |
| TypeScript | `5.9.3` | Latest stable compatible with `typescript-eslint@8`. TypeScript 7 exists but is not yet supported by the ESLint plugin ecosystem. |

Installed dev dependencies (from `pnpm list --depth=0`):

- `@axe-core/playwright@4.12.1`
- `@eslint/js@10.0.1`
- `@playwright/test@1.61.1`
- `@sveltejs/vite-plugin-svelte@7.2.0`
- `@types/node@24.13.3`
- `eslint@10.7.0`
- `eslint-config-prettier@10.1.8`
- `eslint-plugin-svelte@3.20.0`
- `globals@17.7.0`
- `prettier@3.9.5`
- `prettier-plugin-svelte@4.1.1`
- `svelte-check@4.7.2`
- `typescript@5.9.3`
- `typescript-eslint@8.63.0`
- `vite@8.1.4`
- `vitest@4.1.10`

## Container base images

| Image | Tag | Used in |
|-------|-----|---------|
| Node.js builder | `node:24-bookworm` | `packaging/docker/Dockerfile` (web stage) |
| Go builder | `golang:1.26-bookworm` | `packaging/docker/Dockerfile` (build stage) |
| Runtime base | `debian:bookworm-slim` | `packaging/docker/Dockerfile` (final stage) |

## GitHub Actions

| Action | Version | Used in |
|--------|---------|---------|
| `actions/checkout` | `v4` | all workflows |
| `actions/setup-go` | `v5` | quality, supply-chain |
| `pnpm/action-setup` | `v6` | quality, supply-chain |
| `actions/setup-node` | `v4` | quality, supply-chain |
| `actions/configure-pages` | `v6` | pages |
| `actions/upload-pages-artifact` | `v5` | pages |
| `actions/deploy-pages` | `v5` | pages |
| `actions/upload-artifact` | `v4` | supply-chain |
| `actions/attest-build-provenance` | `v4.1.1` | release |
| `docker/setup-qemu-action` | `v4.2.0` | release |
| `docker/setup-buildx-action` | `v4.2.0` | release |
| `docker/login-action` | `v4.4.0` | release |
| `docker/build-push-action` | `v7.3.0` | release |
| `anchore/sbom-action` | `v0.24.0` | supply-chain |
| `aquasecurity/trivy-action` | `v0.36.0` | supply-chain |
| `github/codeql-action/upload-sarif` | `v4.37.0` | supply-chain |

## CLI / audit tools

| Tool | Invocation | Notes |
|------|------------|-------|
| govulncheck | `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Latest stable. |
| go-licenses | `go run github.com/google/go-licenses/v2@latest check ...` | v2 handles Go 1.26 stdlib packages correctly. |
| pnpm audit | `pnpm --dir web audit --audit-level moderate` | Latest pnpm. |

## Checking for updates

```bash
# Go
go list -m -u all

# Frontend
cd web && pnpm outdated

# GitHub Actions: check each action's latest release on github.com/actions, docker, aquasecurity, anchore.
```

## Known exceptions

- **TypeScript 7.0.2** is available, but `typescript-eslint@8.63.0` requires `typescript >=4.8.4 <6.1.0`. We therefore remain on TypeScript 5.9.3 until the ESLint ecosystem supports TS 7.
