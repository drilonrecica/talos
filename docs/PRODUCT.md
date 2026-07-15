# Binnacle product boundaries

This document is the durable source of truth for Binnacle's product boundaries
and guarantees. Architecture decision records define binding implementation
choices; the [roadmap](../ROADMAP.md) describes non-binding future direction.

## Purpose and audience

Binnacle is a lightweight, self-hosted monitoring dashboard for developers and
small teams operating Docker workloads on one Linux server. It answers whether
the host and logical services are healthy, what changed, and what needs
attention while avoiding a separate observability stack.

Coolify metadata is used when available, but plain Docker and Compose remain
fully useful. The product favors useful defaults and a browser interface over
mandatory YAML or dashboard construction. Its optional bounded Prometheus
endpoint is an interoperability surface, not a second monitoring runtime.

## Product principles

In priority order:

1. Keep CPU, memory, storage, and network overhead bounded and visible.
2. Observe workloads without operating them.
3. Provide useful defaults and degrade explicitly under partial failure.
4. Keep monitoring data private and local by default.
5. Preserve stable logical service history across container replacement.
6. Prefer operational clarity and progressive disclosure over feature breadth.

## Non-goals

Binnacle is not a general observability platform, Docker control plane, shell,
or arbitrary Docker API proxy. It does not provide workload mutation,
distributed tracing, unbounded log indexing, a free-form dashboard/query
language, Kubernetes support, or an external database requirement. It does not
claim automatic root-cause analysis.

## Supported boundary

One Binnacle instance monitors one Linux Docker server. Supported production
targets are Ubuntu 22.04/24.04 and Debian 12/13 on amd64 or arm64 with Docker
Engine 29.5.1 or newer. Production startup fails closed when the daemon release
is older, missing, or malformed; operators must upgrade the host first.
Coolify-managed Docker is the primary deployment path;
Docker Compose is the portable alternative.

Kubernetes, Podman and other runtimes, Windows, macOS, BSD, multi-server
federation, and hosted coordination are outside the current supported boundary.
Future support must not add complexity to the single-server runtime before it
is needed.

## Security, privacy, and read-only guarantees

Binnacle permanently excludes actions that restart, stop, delete, exec into, or
redeploy monitored workloads. It exposes neither a shell nor a generic Docker
API proxy. This is a product and code guarantee, not a sandbox: access to the
Docker socket remains highly privileged even when mounted `:ro`. Hardened
deployments should use a constrained socket proxy.

Core monitoring works without internet access and sends no telemetry by
default. Operational data and credentials remain local. Secrets and personal
token plaintext are never returned after creation through APIs or logs, and
UI-entered secrets are encrypted at rest
using operator-provided key material. Network-facing features must use bounded
I/O, validate destinations, and prevent server-side request forgery.

## Architecture and persistence constraints

Production is one Go process with embedded Svelte/TypeScript assets and one
local SQLite database. Node.js is build-time only. External databases, message
brokers, telemetry services, and other core runtime dependencies are excluded.

The in-memory Metrics Engine owns current state; SQLite owns durable history,
settings, identity, checks, and alerts. Storage uses typed schemas and
forward-only migrations. Stable logical resource identifiers are distinct from
ephemeral container instance identifiers. Raw samples, rollups, retention,
queues, concurrency, API responses, and background work are bounded. Under
pressure, Binnacle reports degradation and may discard stale work rather than
grow without limit or block collection.

See the [architecture decision records](../adr/) for the implementation choices
behind Go, Svelte, SQLite, SSE, identity, retention, bounded degradation, checks,
and alerts.

## Compatibility and release policy

Releases use semantic versioning. Exact version tags are immutable. `stable`
never points to a prerelease; `beta` may point to beta or release-candidate
builds; `edge` is development-only. Until 1.0, documented breaking changes may
occur between minor releases, but released database migrations remain
forward-only and upgrades must preserve supported stored data.

Only published tags are releases. Implemented work on the default branch is
unreleased until qualification succeeds and a tag is published. Release
procedures and evidence live in [operations documentation](operations/), while
historical release records remain immutable evidence.
