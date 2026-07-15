# Dependency policy and inventory

Binnacle keeps its runtime and build dependency sets narrow. Manifest and
workflow files are the authoritative inventories; this document records how to
review them without duplicating version numbers that quickly become stale.

Last reviewed: 2026-07-15

## Sources of truth

| Area | Authoritative files |
| --- | --- |
| Go toolchain and modules | [`go.mod`](../go.mod), [`go.sum`](../go.sum) |
| Frontend toolchain and packages | [`package.json`](../package.json), [`web/package.json`](../web/package.json), [`pnpm-lock.yaml`](../pnpm-lock.yaml) |
| Container base images | [`packaging/docker/Dockerfile`](../packaging/docker/Dockerfile) |
| GitHub Actions | [`.github/workflows/`](../.github/workflows/) |
| Vendored fonts and licenses | [`web/static/fonts/`](../web/static/fonts/), [`landing/assets/fonts/`](../landing/assets/fonts/) |

Production uses a Go binary, SQLite through CGO, and embedded frontend assets.
Node.js and frontend packages are build-time dependencies only. Exact versions
must be read from the files above rather than copied into operational guides.
Docker integration uses the stable split Moby API and client modules rather
than the monolithic daemon module. The production runtime requires Docker
Engine 29.5.1 or newer; update the host before deploying Binnacle.

## Policy

- Pin reproducible toolchain, module, package, container, and CI inputs.
- Apply security updates promptly; take major upgrades during a deliberate
  compatibility pass rather than automatically.
- Add a dependency only when the maintained code it replaces would be riskier
  or materially more complex.
- Require an AGPL-3.0-compatible license. A new license category requires
  explicit review.
- Avoid floating CI references such as `master` or an unqualified major tag
  where a commit or sufficiently precise release pin is practical.
- Record temporary compatibility exceptions next to the relevant manifest or
  in an ADR when they affect architecture.

## Local review

```bash
go list -m all
pnpm --dir web list --depth=0
make vuln
make licenses
make sbom
make scan
```

The last four targets require network access or their documented external
tools. CI runs vulnerability, license, SBOM, and container scan workflows; see
[the supply-chain policy](operations/supply-chain.md).
