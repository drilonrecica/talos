# Binnacle — Product and Technical Specification

> **Status:** Authoritative implementation specification for Binnacle
> **Audience:** Human maintainers, contributors, and autonomous/agentic coding systems  
> **License decision:** AGPL-3.0-only  
> **Product name:** Binnacle
> **Primary deployment:** Coolify-managed Docker service  
> **Primary implementation stack:** Go backend + Svelte 5 (runes) + TypeScript frontend + SQLite  
> **Primary product principle:** Extremely low resource consumption without sacrificing a polished, useful monitoring experience

---

## 0. Document Purpose and Normative Language

This document is the complete product, architecture, engineering, user-experience, security, open-source, and release specification for Binnacle. It is intended to be sufficient for a new engineer or an AI implementation agent that has no access to the conversations that produced these decisions.

Implementers MUST treat this document as the source of truth unless the project owner explicitly amends it through a recorded architecture or product decision.

The terms **MUST**, **MUST NOT**, **REQUIRED**, **SHOULD**, **SHOULD NOT**, and **MAY** are used normatively:

- **MUST / REQUIRED**: mandatory for conformance.
- **MUST NOT**: prohibited.
- **SHOULD**: expected unless a documented, compelling reason exists.
- **SHOULD NOT**: generally prohibited unless justified and documented.
- **MAY**: optional.

When a lower-level implementation detail is not yet fixed, this document specifies the boundary, invariants, and acceptance criteria that the implementation must satisfy. Where exact defaults are provided, they are normative defaults and must be user-configurable only where this document explicitly says so.

---

## 1. Executive Summary

Binnacle is a fully open-source, self-hosted monitoring product for Linux servers running Docker workloads, with first-class awareness of Coolify-managed applications and services.

The core product promise is:

> **Lightweight, Coolify-aware monitoring for Docker servers.**

The product should give a developer running one or more applications on a VPS an immediate answer to three questions:

1. Is the server healthy?
2. Are the applications and services healthy?
3. What changed, and what needs attention?

Binnacle is intentionally not a general-purpose enterprise observability platform. It does not require Prometheus, Grafana, Loki, Elasticsearch, InfluxDB, Redis, PostgreSQL, Kubernetes, or any remote SaaS dependency. Its core runtime is one Go process, one SQLite database, and one embedded Svelte 5 frontend.

Binnacle is designed to be:

- extremely lightweight;
- simple to install through Coolify;
- useful without configuration;
- configurable through a built-in admin/settings UI;
- read-only with respect to the monitored host and Docker workloads;
- privacy-preserving and local-first;
- visually distinctive, professional, and pleasant to use;
- information-rich without becoming overwhelming;
- inspired by the immediacy and density of Glances, but not a copy of its UI or product model;
- reliable under partial failure;
- transparent about its own overhead.

The initial product serves one monitored server per Binnacle instance. The code architecture must preserve a clean path to a future central dashboard with lightweight agents, but multi-server federation is not part of the first alpha.

---

## 2. Product Vision, Audience, and Positioning

### 2.1 Primary product goal

Binnacle will become:

> **The fastest, simplest, most visually distinctive self-hosted monitoring dashboard for Docker and Coolify servers.**

The product must optimize for practical monitoring of one VPS and many small or medium Dockerized applications. It should not optimize for large Kubernetes fleets, hyperscale ingestion, distributed tracing, or arbitrary enterprise observability pipelines.

### 2.2 Target audience

Primary audience:

- developers running Docker on one or more Linux VPSs;
- developers deploying applications through Coolify;
- solo developers and small teams that want history and diagnostics without operating a monitoring stack.

Secondary audience:

- agencies hosting several client projects;
- homelab operators using Docker;
- small infrastructure teams that value low overhead and local data.

Explicitly not the primary audience:

- Kubernetes platform teams;
- large enterprise observability organizations;
- users requiring distributed tracing, petabyte-scale logs, or complex telemetry pipelines.

### 2.3 Product philosophy

Binnacle follows **smart defaults with optional advanced settings**.

The intended path is:

```text
Deploy
→ create admin account
→ verify host and Docker access
→ enter dashboard
→ useful data appears automatically
```

The default user experience must not require:

- YAML configuration;
- Prometheus configuration;
- exporter configuration;
- Grafana dashboards;
- manual container discovery;
- manual project grouping;
- external storage configuration.

Advanced users may override sampling, retention, alerts, integration credentials, security controls, and display preferences from Settings or declarative configuration where appropriate.

### 2.4 Coolify-first, not Coolify-only

Binnacle is Coolify-first. This means it should understand and present:

- projects;
- environments;
- applications;
- services;
- multi-container resources;
- Coolify infrastructure containers;
- deployments and container replacement patterns;
- domains and metadata when optional Coolify API enrichment is configured.

However, Binnacle MUST also function on a plain Docker host. Unmanaged Docker containers must remain visible and useful.

### 2.5 Long-term architecture direction

The roadmap direction is:

```text
Phase 1: one Binnacle instance monitors one server
Phase 2: lightweight agents report to a central self-hosted dashboard
Phase 3: optional hosted/cloud coordination may be considered
```

Phase 2 and Phase 3 MUST NOT impose unnecessary complexity on Phase 1. Internal identifiers, module boundaries, and API models should avoid assumptions that make multi-server support impossible later.

---

## 3. Non-Goals

The following are explicit non-goals for the initial product and should be treated as scope protection:

- replacing Grafana as a general dashboard builder;
- becoming a Prometheus-compatible time-series database internally;
- storing or indexing all application logs;
- providing distributed tracing;
- providing arbitrary SQL-like metric query languages;
- supporting Kubernetes in v1;
- supporting Windows, macOS, or BSD hosts in v1;
- acting as a Docker control plane;
- restarting, stopping, deleting, exec-ing into, or redeploying containers;
- providing a shell or terminal;
- exposing generic Docker API proxy endpoints;
- supporting arbitrary dynamic plugins in-process;
- requiring an external database;
- requiring an internet connection for the core dashboard;
- implementing a full incident-management product like PagerDuty;
- implementing a Grafana-style freeform dashboard canvas;
- automatically claiming root cause analysis.

These non-goals are essential to maintaining low resource usage, security, simplicity, and product clarity.

---

## 4. Competitive Context: Binnacle vs. Glances

Binnacle is influenced by the immediacy and operational usefulness of Glances, but it is not intended to clone Glances.

### 4.1 Areas where Binnacle should be stronger

Binnacle is intended to provide capabilities that Glances does not natively prioritize in the same integrated way:

- built-in historical metrics stored locally in SQLite;
- automatic rollups and tiered retention;
- Coolify-aware grouping of containers into logical applications and services;
- stable resource history across container replacement and redeployment;
- deployment annotations and deployment-aware grace periods;
- combined service metrics with expandable component metrics;
- built-in historical charts without requiring an external time-series stack;
- a polished browser-first user interface;
- service-first overview rather than raw process/container-first presentation;
- resource-level correlation between charts, lifecycle events, deployment events, health transitions, and on-demand logs;
- a permanently read-only operations philosophy;
- built-in self-observation showing Binnacle CPU, memory, write latency, queue depth, database size, dropped samples, and collector duration;
- synthetic demo mode for frontend development, demos, documentation, and tests;
- Coolify one-click installation as a first-class distribution path;
- explicit local-first/no-telemetry trust positioning.

### 4.2 Areas where Glances remains stronger

Binnacle should not attempt to match Glances in every area. Glances has major advantages that Binnacle intentionally does not target initially:

- mature terminal/TUI experience;
- broad operating system support;
- broad hardware and sensor support;
- many exporters and integrations;
- long operational maturity;
- mature ecosystem and community experience;
- convenient SSH-oriented usage;
- broader host-level plugin coverage.

Binnacle must win through focus, not breadth.

---

## 5. Core Product Principles

Every implementation decision should be checked against these principles, in this order:

1. **Minimal overhead** — monitoring must not become a meaningful part of server load.
2. **Read-only safety** — Binnacle observes; it does not operate workloads.
3. **Useful defaults** — default installation should immediately provide value.
4. **Local-first privacy** — metrics remain on the server by default.
5. **Graceful degradation** — partial failures must not collapse the whole product.
6. **Coolify awareness** — present logical applications and services, not only containers.
7. **Operational clarity** — the UI should answer what is wrong and what changed.
8. **Progressive disclosure** — simple overview first, dense technical detail on demand.
9. **Transparent performance** — Binnacle must measure and expose its own resource cost.
10. **Scope discipline** — reject features that turn Binnacle into a generic observability suite.

---

## 6. Licensing, Open Source, and Governance

### 6.1 License

The project will use **AGPL-3.0-only**.

The intent is to ensure that modified network-served versions remain source-available under the AGPL terms. The license does not prohibit commercial use. Commercial hosting, support, consulting, and resale are legally possible subject to the license requirements.

Required repository files:

```text
LICENSE
NOTICE or equivalent project notice if needed
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SECURITY.md
GOVERNANCE.md
```

The source headers and package metadata must use the SPDX identifier:

```text
AGPL-3.0-only
```

### 6.2 Contribution model

The project is founder-led. The repository remains under the project owner's personal GitHub account, not a separate organization.

The contribution model is lightweight but explicit:

- ordinary bug fixes and small features use pull requests;
- major architectural changes use a lightweight RFC or ADR process;
- contributions use Developer Certificate of Origin sign-off;
- copyright assignment is not required;
- external PRs should generally be squash-merged;
- Conventional Commits are used for commit and release automation.

Recommended Conventional Commit types:

```text
feat
fix
perf
refactor
docs
test
build
ci
chore
```

### 6.3 Trademark and identity

The open-source software license and the Binnacle name/logo are separate concerns. The product name and visual identity should be protected by an explicit trademark/name-use policy before the first stable release.

---

## 7. Supported Platforms and Compatibility

### 7.1 Official v1 host support

Officially supported:

- Ubuntu 22.04 LTS;
- Ubuntu 24.04 LTS;
- Debian 12;
- Debian 13;
- amd64;
- arm64;
- Docker Engine 24+;
- Docker Compose deployments;
- Coolify-managed Docker hosts.

Other Linux distributions MAY work, but must be labeled community-supported until CI and integration fixtures prove support.

### 7.2 Explicitly unsupported in v1

- Kubernetes;
- Podman;
- generic containerd without Docker API compatibility;
- Windows hosts;
- macOS hosts;
- BSD hosts.

### 7.3 Browser support

Official frontend support covers the latest two stable major versions of:

- Chrome;
- Firefox;
- Edge;
- Safari.

Current mobile Safari and mobile Chrome must support the responsive overview and core incident/resource views.

Internet Explorer and obsolete embedded browsers are not supported.

---

## 8. Deployment Model

### 8.1 Primary deployment path

The primary supported deployment is a Docker container managed by Coolify.

Distribution channels, in priority order:

1. Coolify one-click service template;
2. Docker Compose;
3. GHCR container image;
4. native `.deb` package later;
5. native `.rpm` package later;
6. standalone static binary later.

The Docker/Coolify deployment must provide nearly full product capability. Native deployment may retain one operational advantage: it can continue monitoring the host if Docker itself is unavailable.

### 8.2 Required host access for Docker deployment

The standard deployment needs read access to host telemetry and Docker metadata:

```text
/host/proc       ← bind mount of host /proc, read-only
/host/sys        ← bind mount of host /sys, read-only
/var/run/docker.sock ← Docker API socket
persistent data volume mounted at /var/lib/binnacle
```

Example conceptual Compose structure:

```yaml
services:
  binnacle:
    image: ghcr.io/drilonrecica/binnacle:stable
    restart: unless-stopped
    read_only: true
    privileged: false
    volumes:
      - binnacle-data:/var/lib/binnacle
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /etc/os-release:/host/etc/os-release:ro
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      BINNACLE_HOST_PROC: /host/proc
      BINNACLE_HOST_SYS: /host/sys
      BINNACLE_DATA_DIR: /var/lib/binnacle

volumes:
  binnacle-data:
```

The actual production template must be tested on supported Coolify and Docker setups.

### 8.3 Docker socket security

Important invariant:

> A `:ro` filesystem mount of the Docker Unix socket does not make Docker API operations logically read-only.

Therefore:

- Binnacle code MUST contain no Docker mutation code paths;
- Binnacle MUST NOT expose arbitrary Docker API paths;
- Binnacle MUST NOT accept user-supplied Docker API method/path combinations;
- Docker client wrappers should expose only read methods to the rest of the codebase;
- the recommended hardened deployment SHOULD use a restricted socket proxy with an allowlist of read endpoints;
- the direct socket path is allowed as the simple first-release default.

### 8.4 Persistent storage

The Coolify template defaults to a named persistent volume.

Default data directory:

```text
/var/lib/binnacle
```

Expected contents:

```text
/var/lib/binnacle/
├── binnacle.db
├── binnacle.db-wal
├── binnacle.db-shm
└── runtime/        # ephemeral runtime artifacts if needed
```

No built-in backups are included in v1. The UI and documentation must explicitly warn that deleting the volume removes historical data and application configuration stored in SQLite.

### 8.5 Release channels

Supported container tags:

```text
stable
beta
edge
<major>
<major>.<minor>
<major>.<minor>.<patch>
<semver prerelease>
```

Rules:

- `stable` MUST NOT point to an alpha or beta build;
- `beta` MAY point to beta and release-candidate builds;
- `edge` tracks development builds and is not recommended for production;
- exact semantic version tags are immutable.

Automatic redeployment is disabled by default. Binnacle may detect and display an available update, but v1 MUST NOT mutate its own Coolify resource or replace its own container.

---

## 9. Technology Stack

### 9.1 Backend

Required:

- Go;
- one main executable;
- `net/http`-compatible HTTP stack or a minimal router built around it;
- Docker Engine API integration through a maintained Go client or a deliberately narrow internal client;
- SQLite;
- embedded frontend assets;
- Server-Sent Events for live updates.

The exact Go version is pinned by `go.mod` and CI. The project should use a current stable Go release at implementation start and keep the minimum supported Go toolchain explicit.

### 9.2 Frontend

Required:

- Svelte 5;
- runes-based state model;
- TypeScript;
- Vite;
- pnpm;
- ESLint;
- Prettier;
- Vitest;
- Playwright.

Production must ship no Node.js runtime. The compiled frontend is embedded into the Go binary/image.

### 9.3 UI implementation style

Use a custom Binnacle design system.

Use lightweight headless accessibility primitives only for behavior-heavy components such as:

- dialogs;
- menus;
- popovers;
- tooltips;
- tabs;
- comboboxes.

Do not adopt a full visual component framework that makes Binnacle look generic or adds excessive bundle weight.

### 9.4 Chart implementation

Use a lightweight chart primitive/foundation, with Binnacle-owned presentation for:

- chart frames;
- axis styling;
- tooltips;
- range controls;
- incident/deployment annotations;
- empty states;
- loading states;
- summary statistics.

Required chart types initially:

- time-series line chart;
- time-series area chart;
- stacked resource usage where meaningful;
- small sparklines;
- event/deployment markers.

Do not add pie charts, 3D charts, decorative gauges, or continuously animated dashboards.

---

## 10. High-Level Architecture

### 10.1 One binary, internally modular

Binnacle ships as one binary and one container/process, but internally consists of independent components:

```text
Host Collector
Docker Collector
Coolify Resolver
Health/Collector State
        ↓
Metrics Engine
        ↓
In-memory Current State
        ├──→ SSE Live Stream
        ├──→ Read API
        └──→ Persistence Batch Queue
                  ↓
             SQLite Writer
                  ↓
             Rollup Worker
                  ↓
             Retention Worker

Additional independent modules:
- Auth/session service
- Settings service
- Event correlator
- Alert engine (post-alpha)
- Health-check engine (post-alpha)
- Notification adapters (post-alpha)
- Diagnostics generator
- Update-check service (optional)
```

### 10.2 Metrics Engine as single source of truth

The Metrics Engine is mandatory and central to performance.

Responsibilities:

- receive collector samples;
- normalize units;
- compute rates from counters;
- map container instances to logical resources;
- aggregate multi-container resource snapshots;
- maintain current host, container, and resource state in memory;
- publish immutable live snapshots to subscribers;
- create persistence batches at the configured persistence interval;
- avoid duplicate normalization and aggregation work.

Rules:

- live dashboard requests MUST read current data from memory, not SQLite;
- historical queries read SQLite;
- collectors MUST NOT write SQLite directly;
- SSE handlers MUST NOT query SQLite for every live update;
- resource aggregation MUST be computed once per sample cycle and reused.

### 10.3 Concurrency model

The implementation should favor bounded, explicit concurrency:

- one goroutine per long-lived collector stream where appropriate;
- one metrics engine event loop or carefully synchronized state store;
- one SQLite writer goroutine owning write transactions;
- one rollup worker;
- one retention worker;
- bounded worker pools for health checks and other network operations;
- bounded fan-out for SSE clients;
- no unbounded goroutine creation based on user input or container count.

All queues must be bounded.

### 10.4 Cancellation and shutdown

The process must use a root context. On SIGTERM/SIGINT:

1. stop accepting new HTTP connections;
2. stop new collector cycles;
3. finish or cancel in-flight Docker reads within bounded timeout;
4. flush the current persistence batch if possible;
5. checkpoint WAL if safe and useful;
6. close SQLite;
7. exit before the container orchestrator kill timeout.

Recommended default graceful shutdown budget: 15 seconds.

---

## 11. Repository Structure

Required monorepo layout:

```text
/
├── cmd/
│   └── binnacle/
│       └── main.go
├── internal/
│   ├── app/
│   ├── collector/
│   │   ├── host/
│   │   └── docker/
│   ├── metrics/
│   ├── dockerapi/
│   ├── coolify/
│   ├── resources/
│   ├── storage/
│   ├── rollup/
│   ├── retention/
│   ├── events/
│   ├── alerts/
│   ├── checks/
│   ├── notifications/
│   ├── auth/
│   ├── settings/
│   ├── diagnostics/
│   ├── api/
│   └── webembed/
├── web/
│   ├── src/
│   ├── static/
│   ├── tests/
│   └── package.json
├── migrations/
├── packaging/
│   ├── docker/
│   ├── coolify/
│   ├── deb/
│   ├── rpm/
│   └── systemd/
├── docs/
├── scripts/
├── adr/
├── Makefile
├── go.mod
├── pnpm-lock.yaml
├── LICENSE
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── SECURITY.md
└── GOVERNANCE.md
```

Package boundaries should be acyclic. UI/API layers must depend on service interfaces, not collector internals.

---

## 12. Architecture Decision Records

Initial ADRs to create before or alongside implementation:

```text
001 — Go backend
002 — Svelte 5 frontend with runes and TypeScript
003 — Typed SQLite storage
004 — Docker/Coolify primary deployment
005 — Permanently read-only operational model
006 — AGPL-3.0-only licensing
007 — Server-Sent Events for live updates
008 — Single-binary process architecture
009 — Metrics Engine as current-state single source of truth
010 — Stable logical resource and ephemeral container instance identities
011 — Tiered retention and rollups
012 — Coolify-first, Docker-compatible resource model
013 — No telemetry by default
014 — Bounded queues and graceful degradation
```

ADRs are required for future decisions affecting:

- security boundaries;
- storage model;
- public API compatibility;
- licensing;
- deployment model;
- frontend architecture;
- compatibility guarantees;
- major dependencies.

---

## 13. Configuration System

### 13.1 Configuration sources and precedence

Effective configuration precedence for ordinary configurable settings:

```text
1. built-in defaults
2. config file
3. environment variables
4. persisted admin UI overrides
```

Deployment-critical settings that cannot safely change at runtime are not overridden from the admin UI. Examples:

- data directory;
- database path;
- HTTP listen address;
- master encryption key;
- Docker socket path;
- host `/proc` path;
- host `/sys` path.

For every UI-editable setting, the Settings page must display:

- effective value;
- source (`Default`, `Config file`, `Environment`, `Admin override`);
- whether the change applies immediately or requires restart.

### 13.2 Configuration file

Use one human-readable configuration format. TOML is recommended.

Default discovery order:

```text
/etc/binnacle/binnacle.toml
/var/lib/binnacle/binnacle.toml   # optional container-friendly path
```

An explicit environment variable may set an alternate path:

```text
BINNACLE_CONFIG_FILE
```

### 13.3 Core default settings

Normative defaults:

```text
collection.host_interval              2s
collection.container_interval         2s
live.sse_interval                      2s
persistence.raw_interval              10s
retention.preset                       balanced
database.target_budget_bytes           1073741824   # 1 GiB
database.warning_ratio                 0.80
database.critical_ratio                0.95
database.emergency_pause_ratio         0.98
persistence.queue_batch_limit          60            # 10 minutes at 10s persistence interval
charts.max_points_per_series           1000
collection.minimum_interval            1s
docker.max_concurrency                 4
checks.max_concurrency                 8             # post-alpha
logs.max_response_bytes                1048576       # post-alpha, 1 MiB
logs.max_lines                         5000          # post-alpha
sessions.idle_timeout                  12h
sessions.absolute_lifetime             720h          # 30 days
```

### 13.4 Settings application semantics

Live-applicable examples:

- collection interval;
- persistence interval;
- retention preset;
- retention tier durations;
- chart defaults;
- display preferences;
- alert thresholds later;
- notification configuration later.

Restart-required examples:

- data directory;
- listen address;
- Docker socket path;
- host proc/sys paths;
- master key source.

The UI must label restart-required settings. A future native deployment may provide a controlled restart action, but the Coolify v1 deployment must not self-restart or mutate its own deployment. Instead, it must show the required action and link to the Coolify resource where possible.

---

## 14. First-Run Onboarding

### 14.1 Required onboarding flow

On a fresh instance:

```text
1. Establish admin account
2. Confirm access exposure mode (public URL or private access)
3. Verify host metric access
4. Verify Docker API access
5. Detect Coolify/Compose metadata
6. Confirm sampling and retention preset
7. Enter dashboard
```

The flow must be short and must not require notification configuration.

### 14.2 Admin bootstrap

Support both:

- environment variable / Docker secret bootstrap;
- one-time browser setup.

If bootstrap credentials are supplied securely, the account is created automatically.

If no admin exists, one-time setup mode is activated. Setup mode MUST automatically disable after successful admin creation and MUST NOT silently re-enable later.

A fresh public deployment must not permit arbitrary visitors to claim ownership. The implementation must require a one-time setup token supplied through a Docker secret, environment variable, or one-time startup log. The token must be high entropy and expire after successful setup or a bounded time.

### 14.3 Guided diagnostics

Onboarding diagnostics must report independent status for:

```text
Host metrics access
Docker API access
cgroup access
Coolify/Compose detection
Persistent storage writeability
Database initialization
Outbound network availability (informational)
```

Errors must be actionable, e.g.:

```text
Docker access: Failed
Reason: permission denied opening /var/run/docker.sock
Suggested fix: verify the Docker socket mount and socket permissions
Technical details: expandable
```

The entire setup must not fail because one optional integration is unavailable.

### 14.4 Post-setup checklist

After dashboard entry, show a dismissible checklist:

```text
Create admin account          Done
Host monitoring              Done
Docker monitoring            Done / Needs attention
Coolify resources detected   Done / Not detected
Enable first health check    Optional, post-alpha
Configure notifications      Optional, post-alpha
Review alert thresholds      Optional, post-alpha
```

### 14.5 Optional simulated incident tutorial

Demo infrastructure should later support an optional walkthrough:

```text
CPU spike
→ container restart
→ health check failure
→ recovery
```

The walkthrough demonstrates:

- incident timeline;
- chart annotations;
- nearby logs;
- resolution state.

This must be optional and permanently dismissible.

---

## 15. Authentication, Sessions, and Secrets

### 15.1 v1 authentication model

v1 uses one local admin account.

Do not build teams, roles, invitations, or multi-user permissions in v1.

Required:

- Argon2id password hashing;
- secure session cookies;
- HttpOnly cookies;
- Secure cookies when HTTPS is used;
- SameSite Lax or Strict according to flow requirements;
- session rotation after successful login;
- login rate limiting;
- logout current session;
- logout all sessions;
- configurable session idle timeout;
- configurable absolute session lifetime.

Default session settings:

```text
idle timeout: 12 hours
absolute lifetime: 30 days
```

### 15.2 Future authentication options

Post-alpha:

- TOTP;
- trusted-proxy authentication;
- external identity provider integration;
- option to disable local login when external authentication is active.

### 15.3 Deployment security recommendation

Binnacle may be exposed through a normal HTTPS domain, but documentation should recommend stronger access controls such as:

- Tailscale;
- WireGuard;
- Cloudflare Access;
- private reverse proxy;
- IP allowlisting.

### 15.4 Secret storage

Supported secret sources:

- environment variables;
- Docker secrets;
- encrypted SQLite values for secrets entered in the UI.

UI-entered secrets must be encrypted using a master key supplied separately via environment variable or Docker secret.

Rules:

- SQLite must not contain recoverable plaintext secrets;
- secret values are never returned through API responses;
- Settings displays only `Configured`, `Not configured`, `Replace secret`, and `Remove secret`;
- master key rotation needs an explicit documented process before encrypted UI secrets are released broadly.

---

## 16. Privacy, Telemetry, and Offline Operation

### 16.1 Default telemetry policy

Default behavior:

- no product analytics;
- no anonymous usage telemetry;
- no automatic crash uploads;
- no remote metric storage;
- no advertising or tracking code.

The UI should clearly communicate:

> Your metrics stay on this server. No telemetry is being sent.

### 16.2 Optional update checks

Update checks may be enabled or disabled independently. They should send only the minimal information required to check a release channel and must not include server metrics, resource names, domains, IPs, or container metadata.

### 16.3 Offline core

The core product must work offline.

Production frontend must not require:

- external font CDNs;
- external JavaScript CDNs;
- analytics endpoints;
- remote icon sets;
- hosted API services.

All UI assets must be embedded or served locally.

Outbound traffic occurs only for explicitly configured features such as:

- update checks;
- HTTP health checks;
- SMTP;
- notification webhooks;
- optional Coolify API enrichment if the endpoint is not local.

The UI should show which outbound features are enabled.


---

## 17. Host Metrics Collection

### 17.1 Collection strategy

Host metrics should be read directly from Linux kernel interfaces. Binnacle MUST NOT shell out to `top`, `free`, `df`, `iostat`, `docker stats`, or similar commands on every collection cycle.

Primary sources:

```text
/proc/stat
/proc/meminfo
/proc/loadavg
/proc/uptime
/proc/net/dev
/proc/diskstats
/proc/mounts or /proc/self/mountinfo from host mount namespace
/proc/<pid>/stat and related files only for demand-driven process views
/sys/fs/cgroup/*
/sys/class/net/*
/sys/block/*
```

When containerized, paths are resolved through configured host mount prefixes, e.g. `/host/proc` and `/host/sys`.

### 17.2 Host CPU metrics

Required metrics:

- total CPU utilization percentage;
- per-core CPU utilization percentage;
- user percentage;
- system percentage;
- iowait percentage;
- steal percentage;
- number of online CPUs;
- load averages 1m, 5m, 15m.

The collector reads `/proc/stat` at interval boundaries and computes deltas.

For each CPU line, use fields in kernel order:

```text
user nice system idle iowait irq softirq steal guest guest_nice
```

`guest` and `guest_nice` are already included in user/nice accounting and MUST NOT be added again to total CPU time.

Recommended calculations:

```text
idle_all = idle + iowait
non_idle = user + nice + system + irq + softirq + steal
total = idle_all + non_idle

delta_total = total_now - total_prev
delta_idle = idle_all_now - idle_all_prev

cpu_busy_pct = 100 * (delta_total - delta_idle) / delta_total
user_pct     = 100 * delta(user + nice) / delta_total
system_pct   = 100 * delta(system + irq + softirq) / delta_total
iowait_pct   = 100 * delta(iowait) / delta_total
steal_pct    = 100 * delta(steal) / delta_total
```

Guard against zero/negative deltas due to resets or clock anomalies. Invalid cycles must produce a missing sample, not a fabricated zero.

Steal time is operationally important on VPS workloads and must be shown in detailed server views.

### 17.3 Memory metrics

Required:

- MemTotal;
- MemAvailable;
- used bytes;
- used percentage;
- swap total;
- swap used;
- swap percentage;
- cache/buffer informational breakdown when available.

Primary memory usage formula:

```text
used_bytes = MemTotal - MemAvailable
used_pct = used_bytes / MemTotal * 100
```

Binnacle should treat `MemAvailable` as the main available-memory signal rather than `MemFree`.

Cache/buffer details may be shown for diagnostics but should not confuse the primary “used” metric.

### 17.4 Load averages

Read `/proc/loadavg` and expose:

- 1-minute load;
- 5-minute load;
- 15-minute load.

The UI may additionally show load normalized by online CPU count:

```text
normalized_load_1 = load_1 / online_cpus
```

This normalized value is diagnostic, not a replacement for raw load.

### 17.5 Uptime and boot identity

Read `/proc/uptime` for uptime seconds.

Binnacle should derive or record a boot identity to detect host reboot boundaries. A new boot must create an event and break historical continuity where counters reset.

### 17.6 Network metrics

Required host metrics:

- RX bytes per second;
- TX bytes per second;
- RX packets per second;
- TX packets per second;
- receive errors;
- transmit errors;
- dropped RX/TX packets;
- per-interface data;
- host aggregate excluding loopback by default.

Source: `/proc/net/dev` or host equivalent.

Rate formula:

```text
rate = max(0, counter_now - counter_prev) / elapsed_seconds
```

Counter decreases indicate interface reset/recreation and should be treated as a reset boundary, not negative traffic.

Default aggregate excludes `lo`. Virtual/container interfaces may be included in detailed interface view but the host aggregate must avoid obvious double counting. The implementation should maintain an interface classification layer and tests for common Docker bridge/veth patterns.

### 17.7 Disk I/O metrics

Required:

- read bytes per second;
- write bytes per second;
- read operations per second;
- write operations per second;
- I/O time or utilization where meaningful;
- queue/pressure diagnostics where kernel data supports it;
- host iowait from CPU metrics.

Source: `/proc/diskstats` plus `/sys/block` metadata.

Rules:

- exclude loop, ram, and irrelevant synthetic devices by default;
- avoid double counting whole disks and partitions;
- preserve per-device detail;
- host aggregate should represent relevant backing devices once.

Linux diskstats sector counters should be converted according to kernel semantics and tested against known fixtures. Do not guess device sector size behavior; centralize conversion logic and fixture-test it.

### 17.8 Filesystem metrics

Required:

- filesystem total bytes;
- used bytes;
- available bytes;
- used percentage;
- inode total;
- inode used;
- inode percentage;
- mount point;
- filesystem type;
- device/source.

Use filesystem stat calls against host-visible mount paths. Exclude pseudo-filesystems by default from overview, including common entries such as proc, sysfs, tmpfs where not operationally relevant, cgroup mounts, and overlay internals. Provide a detailed view for advanced inspection.

The root filesystem and persistent Binnacle volume filesystem must always be visible.

### 17.9 Host process data

Process explorer is post-alpha and demand-driven.

When implemented:

- default top 25 processes;
- fields: PID, command/name, CPU, memory, user, state, uptime, container association if identifiable;
- strictly read-only;
- no signals, renice, kill, or shell operations;
- detailed `/proc` enumeration should run only while the page is open or at a much slower cadence.

---

## 18. Docker Metrics Collection

### 18.1 Docker API usage

Binnacle should use Docker Engine API operations for:

- list containers;
- inspect containers;
- read stats;
- subscribe to lifecycle events;
- read bounded logs on demand post-alpha;
- read version/system metadata needed for diagnostics.

Binnacle MUST NOT use Docker mutation endpoints.

### 18.2 Event-driven metadata cache

Do not repeatedly inspect static metadata every 2 seconds.

Cache:

- container ID;
- names;
- labels;
- image reference;
- image digest if available;
- Compose metadata;
- network membership;
- health-check configuration/status metadata;
- mounts;
- creation/start timestamps.

Refresh cache on:

- startup discovery;
- Docker lifecycle events;
- periodic low-frequency reconciliation;
- cache miss.

Recommended reconciliation default: every 5 minutes, configurable later if needed.

### 18.3 Required container metrics

For each running container instance:

- host-normalized CPU percentage;
- Docker-style/core CPU percentage or core-equivalent value in detail view;
- memory working set bytes;
- memory current/usage bytes;
- memory limit bytes where finite;
- memory utilization percentage;
- network RX bytes/sec;
- network TX bytes/sec;
- block read bytes/sec;
- block write bytes/sec;
- PID count;
- state;
- health status if Docker health checks exist;
- restart count;
- started-at timestamp.

### 18.4 Container CPU formula

The default UI metric is **host-normalized CPU percentage**: the whole VPS equals 100%.

If Docker stats fields are available:

```text
container_delta = total_usage_now - total_usage_prev
system_delta    = system_cpu_usage_now - system_cpu_usage_prev
online_cpus     = reported online CPUs or host online CPU count

docker_style_pct = (container_delta / system_delta) * online_cpus * 100
host_normalized_pct = docker_style_pct / online_cpus
core_equivalents = docker_style_pct / 100
```

Equivalent simplification:

```text
host_normalized_pct = (container_delta / system_delta) * 100
```

provided the Docker stats system counter semantics are consistent for the platform.

The UI must label conventions clearly:

```text
CPU: 25% of host
Equivalent: 1.0 CPU core
```

If cgroup v2 usage is read directly:

```text
core_equivalents = delta_cpu_usage_seconds / elapsed_wall_seconds
host_normalized_pct = core_equivalents / online_cpus * 100
```

Clamp only for display sanity. Do not hide legitimate multi-core usage in the core-equivalent metric.

### 18.5 Container memory semantics

Primary displayed container memory should represent working set where possible.

For cgroup v2:

```text
current = memory.current
inactive_file = memory.stat[inactive_file]
working_set = max(0, current - inactive_file)
```

Store both:

- `memory_usage_bytes` = raw current usage;
- `memory_working_set_bytes` = adjusted working set.

For a finite container limit:

```text
memory_pct = working_set / limit * 100
```

If the limit is unlimited or greater than practical host memory, use host total memory as the comparison denominator for UI context and label it as host-relative.

### 18.6 Container network metrics

Sum Docker stats network counters across the container's interfaces for container total.

Store rates computed from counter deltas:

```text
rx_bps
tx_bps
```

A container replacement starts a new instance counter series. Resource-level history remains continuous through logical resource aggregation.

### 18.7 Container block I/O

Collect read/write bytes from Docker stats or cgroup v2 `io.stat`.

Store:

```text
block_read_bps
block_write_bps
```

Resource aggregate sums component instance rates for the current logical resource.

### 18.8 PID count

Use Docker stats `pids_stats.current` or cgroup equivalent.

Store current PID count. This metric is useful for leak diagnosis but should not be overemphasized on the overview.

### 18.9 Lifecycle events

Subscribe to Docker events and normalize at least:

```text
container_start
container_stop
container_die
container_destroy
container_restart
container_oom
container_health_status_change
container_create
container_rename
image_change/replacement inference
```

Record normalized events with raw diagnostic context where safe.

### 18.10 OOM detection

Record OOM from:

- Docker `oom` event;
- `State.OOMKilled` when processing a die/replacement event;
- cgroup memory event counters where practical.

Deduplicate equivalent signals into one user-visible OOM event with related raw references.

---

## 19. Coolify-Aware Resource Model

### 19.1 Core identity model

Binnacle distinguishes:

1. **Logical resource** — stable application/service identity across deployments.
2. **Container instance** — one concrete Docker container, ephemeral and replaceable.

This separation is mandatory.

### 19.2 Stable identity precedence

Resource identity should be resolved in this order:

```text
1. Coolify resource UUID, when reliably available
2. Docker Compose project + service identity
3. Stable derived identifier from labels and deployment metadata
4. Manual user-defined mapping
```

Container ID must never be used as the long-term resource identity.

### 19.3 Compose metadata

At minimum, support standard Compose metadata such as:

```text
com.docker.compose.project
com.docker.compose.service
com.docker.compose.container-number
```

Any Coolify-specific label mapping must live in one resolver layer, be fixture-tested, and be resilient to missing fields. Do not couple the application to undocumented Coolify database internals.

### 19.4 Optional Coolify API enrichment

Post-alpha optional integration may use a read-only Coolify API token to enrich:

- project names;
- environments;
- service/application names;
- domains;
- deployment metadata;
- resource UUIDs where not locally available.

The product must continue to monitor if the Coolify API is unavailable.

### 19.5 Resource categories

Automatic categories:

```text
Application
Service
Database
Cache
Worker
Proxy
Coolify infrastructure
Unmanaged container
```

Classification inputs may include:

- Coolify metadata;
- Docker labels;
- Compose service name;
- image reference;
- exposed ports;
- network relationships.

User overrides must persist across redeployments.

### 19.6 Multi-container resources

A logical resource may contain multiple components.

Example:

```text
Directus production
├── directus
├── postgres
├── redis
└── worker
```

The default resource summary shows combined metrics:

```text
CPU: sum of current host-normalized component CPU
RAM: sum of component working sets
Network: sum of component rates
Block I/O: sum of component rates
Status: rollup of component and service health states
```

Expandable component detail shows each container instance/component separately.

### 19.7 Resource status rollup

For alpha, status is primarily based on container/collector state because HTTP health checks are post-alpha.

Recommended rollup precedence:

```text
Down > Degraded > Unknown > Paused > Healthy
```

Rules must be documented and deterministic.

A failing component should attach the event/alert to the most specific component while affecting the parent resource status.

### 19.8 Archived resources

When a logical resource disappears permanently:

```text
status = Archived
last_seen_at = timestamp
history = preserved
alerts = disabled
health checks = paused
```

Archived resources are hidden from default overview and available at:

```text
Resources → Archived
```

Users may purge them manually with explicit confirmation.

---

## 20. Deployment Detection and Correlation

### 20.1 Deployment signal sources

Deployment detection should correlate:

- Coolify deployment events when API enrichment exists;
- image digest/reference changes;
- old container exit plus new container creation for same logical resource;
- Compose project reconciliation patterns;
- startup within an expected replacement window;
- health transition around rollout.

### 20.2 Confidence levels

Normalized deployment classifications:

```text
Confirmed deployment
Likely deployment
Container replacement
```

Only confirmed and likely deployments should activate deployment-aware grace periods for post-alpha alerts/health checks.

### 20.3 Overlapping rollout handling

During rolling/overlapping deployments, old and new instances may coexist.

Resource metrics must reflect actual combined usage during overlap. Do not discard old instance metrics or show only the newest container.

Deployment detail should be able to represent:

```text
Old instance: healthy → draining → stopped
New instance: starting → checking → active
Overlap duration
Peak rollout CPU
Peak rollout memory
```

---

## 21. Collection Scheduling and Data Flow

### 21.1 Default clocks

Normative defaults:

```text
Host collection:          every 2 seconds
Container collection:     every 2 seconds
Live SSE publish:         every 2 seconds
Raw persistence batch:    every 10 seconds
Health checks later:      every 30 seconds
TLS checks later:         every 6 hours
Rollup worker:            every 5 minutes or event-driven equivalent
Retention worker:         hourly by default
```

### 21.2 User configurability

The user may configure host/container collection and persistence intervals through Settings.

Minimum permitted collection interval:

```text
1 second
```

The UI must warn when aggressive intervals increase overhead.

### 21.3 Continuous vs demand-driven collection

Always-on collection:

- host CPU/memory/load;
- filesystem capacity;
- disk I/O;
- host network;
- container CPU/memory/I/O/network;
- container state;
- Docker events;
- collector health;
- health checks later.

Demand-driven or slow-cadence:

- full process enumeration;
- live logs;
- expensive filesystem breakdown;
- repeated deep container inspections;
- optional advanced per-core details if disabled by user.

### 21.4 No duplicate work

A collector sample should be parsed and normalized once. The resulting snapshot should be shared by:

- current-state cache;
- resource aggregation;
- SSE serialization;
- persistence batching;
- alert evaluation later;
- self-metrics.

Avoid re-reading `/proc` or Docker stats separately for each consumer.

---

## 22. Metrics Engine Detailed Contract

### 22.1 Inputs

The Metrics Engine receives typed messages such as:

```text
HostSample
ContainerSample
ContainerMetadataUpdate
DockerLifecycleEvent
CollectorHealthUpdate
Clock/BootBoundaryEvent
```

### 22.2 Internal state

The engine maintains:

```text
CurrentHostSnapshot
CurrentContainerInstanceSnapshots map[containerID]
CurrentResourceSnapshots map[resourceID]
ResourceMembership map[resourceID][]containerID
MetadataCache
CollectorHealth map[collectorName]state
RecentEventRing
BootIdentity
SequenceNumber
```

### 22.3 Snapshot invariants

Every published live snapshot must include:

- monotonic sequence number;
- wall-clock timestamp in UTC;
- host boot identity;
- freshness timestamp per collector domain;
- explicit missing/unknown values rather than zero substitution.

### 22.4 Persistence batches

Every raw persistence interval, the engine creates one immutable batch containing:

- one host sample;
- one sample per active logical resource;
- optional instance-level sample records if retained;
- collector health state changes since last batch;
- pending normalized events.

The SQLite writer consumes batches in order.

### 22.5 Bounded persistence queue

Default maximum queued batches: 60.

At a 10-second persistence interval this represents 10 minutes of queued history.

On overflow:

1. drop the oldest queued batch;
2. increment `dropped_persistence_batches_total`;
3. create/update internal persistence-degraded state;
4. show a visible warning;
5. continue live monitoring.

No queue may grow without bound.

---

## 23. SQLite Storage Architecture

### 23.1 SQLite mode

SQLite should use WAL mode unless platform testing proves a blocker.

Recommended pragmas must be benchmarked and documented. Candidate defaults:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
```

Do not blindly copy performance pragmas without durability tests.

### 23.2 Single writer ownership

One writer component owns write transactions.

Collectors and HTTP handlers must not issue arbitrary writes concurrently.

Settings changes may pass through a storage service that serializes writes with the main writer or uses a controlled transaction boundary.

### 23.3 Typed table strategy

Do not use a generic EAV table like:

```text
timestamp, resource_id, metric_name, value
```

Use typed tables for predictable indexes and compact queries.

### 23.4 Core entity schema

The following schema is normative in structure; exact SQL types may be adapted to SQLite conventions.

#### `hosts`

```text
id TEXT PRIMARY KEY
machine_id_hash TEXT NULL
hostname TEXT NOT NULL
os_name TEXT NULL
os_version TEXT NULL
kernel_version TEXT NULL
arch TEXT NOT NULL
cpu_count INTEGER NOT NULL
created_at INTEGER NOT NULL       # unix milliseconds
last_seen_at INTEGER NOT NULL
```

#### `boot_sessions`

```text
id TEXT PRIMARY KEY
host_id TEXT NOT NULL REFERENCES hosts(id)
boot_identity TEXT NOT NULL
started_at INTEGER NOT NULL
ended_at INTEGER NULL
UNIQUE(host_id, boot_identity)
```

#### `resources`

```text
id TEXT PRIMARY KEY
host_id TEXT NOT NULL REFERENCES hosts(id)
stable_key TEXT NOT NULL
source_kind TEXT NOT NULL          # coolify, compose, derived, manual, unmanaged
source_external_id TEXT NULL
name TEXT NOT NULL
project_name TEXT NULL
environment_name TEXT NULL
category TEXT NOT NULL
status TEXT NOT NULL
archived_at INTEGER NULL
first_seen_at INTEGER NOT NULL
last_seen_at INTEGER NOT NULL
user_category_override TEXT NULL
UNIQUE(host_id, stable_key)
```

#### `container_instances`

```text
id TEXT PRIMARY KEY                # Docker container ID
resource_id TEXT NULL REFERENCES resources(id)
name TEXT NOT NULL
compose_project TEXT NULL
compose_service TEXT NULL
image_ref TEXT NULL
image_digest TEXT NULL
created_at INTEGER NOT NULL
started_at INTEGER NULL
stopped_at INTEGER NULL
destroyed_at INTEGER NULL
exit_code INTEGER NULL
oom_killed INTEGER NOT NULL DEFAULT 0
metadata_json TEXT NULL            # sanitized stable metadata only
```

Indexes:

```text
container_instances(resource_id, started_at)
container_instances(stopped_at)
```

### 23.5 Raw host sample schema

#### `host_samples_10s`

```text
ts INTEGER NOT NULL
host_id TEXT NOT NULL
boot_session_id TEXT NOT NULL
cpu_busy_pct REAL NULL
cpu_user_pct REAL NULL
cpu_system_pct REAL NULL
cpu_iowait_pct REAL NULL
cpu_steal_pct REAL NULL
load_1 REAL NULL
load_5 REAL NULL
load_15 REAL NULL
memory_used_bytes INTEGER NULL
memory_available_bytes INTEGER NULL
memory_used_pct REAL NULL
swap_used_bytes INTEGER NULL
swap_used_pct REAL NULL
disk_read_bps REAL NULL
disk_write_bps REAL NULL
disk_read_iops REAL NULL
disk_write_iops REAL NULL
network_rx_bps REAL NULL
network_tx_bps REAL NULL
network_rx_errors_delta INTEGER NULL
network_tx_errors_delta INTEGER NULL
PRIMARY KEY(host_id, ts)
```

### 23.6 Raw resource sample schema

#### `resource_samples_10s`

```text
ts INTEGER NOT NULL
resource_id TEXT NOT NULL
cpu_host_pct REAL NULL
cpu_core_equiv REAL NULL
memory_working_set_bytes INTEGER NULL
memory_usage_bytes INTEGER NULL
memory_limit_bytes INTEGER NULL
memory_pct REAL NULL
network_rx_bps REAL NULL
network_tx_bps REAL NULL
block_read_bps REAL NULL
block_write_bps REAL NULL
pids INTEGER NULL
active_instance_count INTEGER NOT NULL
PRIMARY KEY(resource_id, ts)
```

### 23.7 Optional instance sample schema

For alpha, retaining instance-level metrics is permitted if benchmarked within storage targets. Recommended:

#### `container_instance_samples_10s`

```text
ts INTEGER NOT NULL
container_instance_id TEXT NOT NULL
cpu_host_pct REAL NULL
cpu_core_equiv REAL NULL
memory_working_set_bytes INTEGER NULL
memory_usage_bytes INTEGER NULL
network_rx_bps REAL NULL
network_tx_bps REAL NULL
block_read_bps REAL NULL
block_write_bps REAL NULL
pids INTEGER NULL
PRIMARY KEY(container_instance_id, ts)
```

If storage benchmarks show unacceptable growth, instance-level raw retention may be shorter than logical resource retention, but this must be explicit and documented.

### 23.8 Filesystem sample schema

#### `filesystem_samples_1m`

Filesystem capacity changes slowly and need not be stored every 10 seconds.

```text
ts INTEGER NOT NULL
host_id TEXT NOT NULL
mount_key TEXT NOT NULL
mount_point TEXT NOT NULL
fs_type TEXT NULL
total_bytes INTEGER NULL
used_bytes INTEGER NULL
available_bytes INTEGER NULL
used_pct REAL NULL
inodes_total INTEGER NULL
inodes_used INTEGER NULL
inodes_used_pct REAL NULL
PRIMARY KEY(host_id, mount_key, ts)
```

Default persistence cadence: 1 minute.

### 23.9 Interface sample schema

#### `network_interface_samples_1m`

```text
ts INTEGER NOT NULL
host_id TEXT NOT NULL
interface_name TEXT NOT NULL
rx_bps REAL NULL
tx_bps REAL NULL
rx_packets_ps REAL NULL
tx_packets_ps REAL NULL
rx_errors_delta INTEGER NULL
tx_errors_delta INTEGER NULL
rx_drops_delta INTEGER NULL
tx_drops_delta INTEGER NULL
PRIMARY KEY(host_id, interface_name, ts)
```

### 23.10 Event schema

#### `events`

```text
id TEXT PRIMARY KEY
ts INTEGER NOT NULL
host_id TEXT NOT NULL
resource_id TEXT NULL
container_instance_id TEXT NULL
type TEXT NOT NULL
severity TEXT NOT NULL               # info, warning, critical
summary TEXT NOT NULL
details_json TEXT NULL
correlation_key TEXT NULL
source TEXT NOT NULL
created_at INTEGER NOT NULL
```

Indexes:

```text
events(ts DESC)
events(resource_id, ts DESC)
events(type, ts DESC)
events(correlation_key, ts DESC)
```

### 23.11 Collector health schema

#### `collector_state_events`

```text
id TEXT PRIMARY KEY
ts INTEGER NOT NULL
collector_name TEXT NOT NULL
previous_state TEXT NULL
new_state TEXT NOT NULL              # healthy, degraded, down, unknown
reason_code TEXT NULL
message TEXT NULL
```

### 23.12 Settings schema

#### `settings`

```text
key TEXT PRIMARY KEY
value_json TEXT NOT NULL
updated_at INTEGER NOT NULL
updated_by TEXT NULL
```

Secrets must not be stored here as plaintext.

#### `encrypted_secrets`

```text
key TEXT PRIMARY KEY
ciphertext BLOB NOT NULL
nonce BLOB NOT NULL
algorithm TEXT NOT NULL
key_version INTEGER NOT NULL
updated_at INTEGER NOT NULL
```

### 23.13 Session schema

#### `users`

```text
id TEXT PRIMARY KEY
username TEXT UNIQUE NOT NULL
password_hash TEXT NOT NULL
created_at INTEGER NOT NULL
updated_at INTEGER NOT NULL
```

#### `sessions`

```text
id_hash TEXT PRIMARY KEY
user_id TEXT NOT NULL
created_at INTEGER NOT NULL
last_seen_at INTEGER NOT NULL
expires_at INTEGER NOT NULL
absolute_expires_at INTEGER NOT NULL
revoked_at INTEGER NULL
user_agent_hash TEXT NULL
ip_prefix_hash TEXT NULL
```

Store session token hashes, not plaintext tokens.

---

## 24. Rollups and Retention

### 24.1 Retention presets

#### Minimal

```text
10-second samples: 12 hours
1-minute rollups: 7 days
15-minute rollups: 90 days
1-hour rollups: 1 year
```

#### Balanced — default

```text
10-second samples: 48 hours
1-minute rollups: 30 days
15-minute rollups: 1 year
1-hour rollups: indefinite
```

#### Long-term

```text
10-second samples: 7 days
1-minute rollups: 90 days
15-minute rollups: 2 years
1-hour rollups: indefinite
```

Advanced users may override each tier.

### 24.2 Rollup statistics

For ordinary infrastructure metrics store:

- minimum;
- maximum;
- average;
- sample count.

For selected latency metrics post-alpha also store:

- p95.

Do not store p50/p99 for every metric by default.

### 24.3 Rollup table pattern

For host and resource rollups, use typed rollup tables such as:

```text
host_rollups_1m
host_rollups_15m
host_rollups_1h
resource_rollups_1m
resource_rollups_15m
resource_rollups_1h
```

Each metric column expands into `_min`, `_max`, `_avg`, plus `sample_count` per row. To avoid excessive width, related metrics may be split by domain if benchmarks or migration ergonomics justify it, but the schema must remain typed and query-efficient.

### 24.4 Rollup correctness

Rollups must be idempotent.

Recommended approach:

- process closed time buckets only;
- use deterministic bucket start timestamps;
- `INSERT ... ON CONFLICT DO UPDATE` from source aggregates;
- never partially delete source tier before destination tier is confirmed.

### 24.5 Missing samples

Rollups must not treat missing data as zero.

`sample_count` is required so the UI can distinguish complete and partial buckets.

### 24.6 Retention deletion order

Deletion runs only after downstream rollups exist.

Example for raw data:

1. ensure all eligible 1-minute buckets are rolled up;
2. delete raw rows older than raw retention cutoff in bounded batches;
3. yield between batches to avoid long write locks.

### 24.7 Database budget

Default target budget:

```text
1 GiB
```

Threshold behavior:

```text
80% of budget: warning
95% of budget: critical; run aggressive expired-data cleanup
98% of budget: pause highest-resolution raw persistence if required
```

Priority when constrained:

1. preserve settings and auth data;
2. preserve recent events and incidents;
3. preserve recent aggregates;
4. preserve recent raw samples;
5. sacrifice oldest queued raw persistence first.

Binnacle must never silently delete still-in-retention data merely to satisfy the soft target. Emergency behavior must be documented and surfaced in the UI.

---

## 25. Database Failure and Degradation Behavior

### 25.1 Principle

Database failure must not make live monitoring disappear.

### 25.2 Failure sequence

If SQLite writes fail because of locking, disk full, I/O error, or corruption:

1. keep collectors running;
2. keep in-memory current state updated;
3. keep SSE live updates active;
4. enqueue persistence batches up to the bounded limit;
5. retry writes with exponential backoff and jitter;
6. drop oldest queued batches on overflow;
7. show `History persistence degraded` prominently;
8. update Database Writer collector health;
9. create an internal event;
10. recover automatically when safe.

### 25.3 Corruption behavior

On corruption detection:

- do not crash-loop;
- stop historical writes;
- serve live state;
- serve historical reads only if SQLite can safely do so;
- show a critical diagnostic state;
- provide recovery instructions;
- do not automatically delete or recreate the database.

### 25.4 Clock changes

Use monotonic elapsed time for rate calculations inside the process. Use UTC wall-clock timestamps for persisted/event data.

If wall clock moves significantly backward or forward:

- record a clock-change event;
- avoid generating negative rates;
- start a new sampling continuity segment if necessary;
- preserve explicit chart gaps rather than interpolating misleading data.

---

## 26. Live Updates and SSE

### 26.1 Protocol

Use Server-Sent Events for server-to-browser live metrics.

Default endpoint:

```text
GET /api/v1/live
Content-Type: text/event-stream
```

### 26.2 Event types

Initial event types:

```text
snapshot
event
collector_state
heartbeat
```

Example conceptual frame:

```text
event: snapshot
id: 12452
data: { ... compact JSON ... }
```

### 26.3 Snapshot payload goals

A live snapshot should contain only data needed by visible overview/resource views. Do not resend large metadata blobs every 2 seconds.

Use IDs and stable metadata caches on the frontend.

Suggested structure:

```json
{
  "seq": 12452,
  "ts": "2026-07-11T12:00:00.000Z",
  "host": {
    "cpuPct": 12.4,
    "memoryUsedBytes": 2684354560,
    "memoryPct": 32.7,
    "load1": 0.42,
    "networkRxBps": 12043,
    "networkTxBps": 8344
  },
  "resources": [
    {
      "id": "res_...",
      "cpuHostPct": 1.7,
      "memoryBytes": 186646528,
      "rxBps": 1200,
      "txBps": 850,
      "status": "healthy"
    }
  ],
  "collectors": {
    "host": "healthy",
    "docker": "healthy",
    "coolify": "healthy",
    "storage": "healthy"
  }
}
```

### 26.4 Backpressure

Each SSE client must have a bounded output buffer.

If a client cannot keep up:

- drop intermediate snapshots and keep the newest snapshot;
- never block collectors or the Metrics Engine;
- preserve discrete event messages where feasible;
- disconnect persistently slow clients.

### 26.5 Heartbeats

Send SSE comment or heartbeat events often enough to keep reverse proxies from closing idle streams. Recommended 15–30 seconds.

---

## 27. HTTP API

### 27.1 API status

Selected read-only endpoints are exposed under:

```text
/api/v1/
```

Before 1.0, these endpoints are experimental unless explicitly stabilized. The built-in frontend may use them, but compatibility guarantees are limited during `0.x`.

### 27.2 Alpha authentication

Alpha API access uses authenticated browser sessions.

Read-only API tokens are post-alpha.

### 27.3 Response conventions

- JSON;
- UTF-8;
- timestamps in RFC 3339 UTC in API responses;
- bytes represented as integer bytes;
- rates represented as bytes/sec or ops/sec with explicit field names;
- percentages represented as `0..100`, not `0..1`;
- nullable measurements use JSON `null`, not zero.

Error envelope:

```json
{
  "error": {
    "code": "invalid_time_range",
    "message": "The requested start time must be before the end time.",
    "details": {}
  }
}
```

### 27.4 Initial endpoints

```text
GET /api/v1/server
GET /api/v1/resources
GET /api/v1/resources/{id}
GET /api/v1/metrics
GET /api/v1/events
GET /api/v1/collector-health
GET /api/v1/live
GET /api/v1/settings
PATCH /api/v1/settings              # authenticated admin, state-changing but not operational control
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/logout-all
```

Post-alpha:

```text
GET /api/v1/incidents
GET /api/v1/checks
GET /api/v1/logs
GET /api/v1/processes
POST /api/v1/api-tokens
DELETE /api/v1/api-tokens/{id}
```

### 27.5 Metrics query contract

Example:

```text
GET /api/v1/metrics?scope=resource&id=res_123&metrics=cpu,memory&from=...&to=...
```

The backend automatically selects storage resolution.

Resolution mapping default:

```text
≤ 2 hours: 10-second raw
≤ 48 hours: 1-minute rollup
≤ 60 days: 15-minute rollup
> 60 days: 1-hour rollup
```

The server may choose a coarser tier to remain under approximately 1000 points per series.

The browser should not need to choose raw/rollup tables.

### 27.6 API tokens — post-alpha

Read-only personal API tokens should support:

```text
name
creation time
last-used time
optional expiry
scopes
revoke
```

Initial scopes:

```text
server:read
resources:read
metrics:read
events:read
incidents:read
```

Token values are shown once. Store only secure hashes.

---

## 28. Frontend Information Architecture

### 28.1 Primary navigation

```text
Overview
Resources
Server
Events
Checks
Settings
```

`Checks` may be hidden or marked unavailable until health checks are implemented post-alpha.

### 28.2 Resources navigation

```text
Projects
Applications
Services
Infrastructure
Unmanaged containers
Archived
```

Containers are not top-level navigation because they are implementation details. Container detail is accessible from resource detail.

### 28.3 Overview goal

The first screen must answer within five seconds:

1. Is the VPS healthy?
2. Are applications/services healthy?
3. What needs attention?

Recommended layout:

```text
Top health summary
- server health
- CPU
- RAM
- disk
- active warnings/incidents later

Applications/resources
- status
- response time later
- CPU
- memory
- last deployment when available

Infrastructure
- Coolify
- proxy
- internal DB
- Redis

Recent events
- restart
- deployment
- OOM
- collector degradation
```

Do not make the overview resemble a dense cockpit.

### 28.4 Resource detail page

Required alpha sections:

```text
Header: name, category, status, project/environment context
Current metrics summary
Time range selector
CPU chart
Memory chart
Network chart
Block I/O chart when relevant
Deployment/replacement annotations
Component list for multi-container services
Recent events
Metadata/details drawer
```

Post-alpha additions:

```text
health checks
incident timeline
related logs
alert rules
```

### 28.5 Server page

Sections:

- current CPU and per-core detail;
- CPU composition including iowait and steal;
- memory and swap;
- load averages;
- disk I/O;
- filesystems and inode usage;
- network aggregate and interfaces;
- uptime and boot events;
- top processes post-alpha;
- collector health.

### 28.6 Events page

Filters:

- time range;
- resource;
- event type;
- severity;
- source.

Events should be chronological, concise, and expandable for technical detail.

### 28.7 Settings hierarchy

Recommended:

```text
Settings
├── General
├── Collection
├── Retention & Storage
├── Authentication
├── Integrations
│   ├── Coolify API (post-alpha)
│   └── Notifications (post-alpha)
├── Alerts (post-alpha)
├── Privacy & Network
├── Appearance
├── System
│   ├── Monitor health
│   ├── Collector health
│   ├── Database status
│   └── Version/update status
└── Diagnostics
```

---

## 29. Visual Design Direction

### 29.1 Design concept

The design direction is:

> **A modern telemetry console with a terminal soul.**

It must be:

- professional;
- operationally credible;
- visually distinctive;
- energetic without looking childish;
- information-rich without clutter;
- inspired by Glances' immediacy, not its exact visual language.

### 29.2 Page personality distribution

```text
Overview: expressive, spatial, memorable
Resource detail: calm, analytical, chart-focused
Logs/processes: dense, monospace-friendly, Glances-inspired
Settings: conventional, clean, predictable
```

### 29.3 Avoid

- generic dashboard template appearance;
- excessive gradients;
- neon cyberpunk styling;
- glowing everything;
- oversized cards with little information;
- radial gauges;
- 3D charts;
- decorative motion;
- color-only status.

### 29.4 Themes

Support:

```text
System
Dark
Light
```

Dark mode should be the signature experience but not pure black hacker-terminal styling.

Dark theme characteristics:

- deep neutral surfaces;
- subtly tinted panels;
- restrained borders;
- clear telemetry contrast;
- limited glow reserved for live/exceptional states.

Light theme must be designed intentionally, not generated by naive inversion.

### 29.5 Density modes

Support:

```text
Comfortable
Compact
```

Comfortable:

- larger spacing;
- summary labels;
- overview-friendly.

Compact:

- denser tables;
- more simultaneous metrics;
- keyboard-efficient;
- closer to Glances' operational density.

### 29.6 Typography

Use a distinctive but readable sans-serif for UI and a monospace face only for:

- IDs;
- container names;
- paths;
- logs;
- technical snippets.

All fonts must be local/embedded or use a robust system stack. No external font CDN.

### 29.7 Status representation

States:

```text
Healthy
Degraded
Down
Paused
Unknown
Archived
Deploying
```

Always combine:

- color;
- icon;
- explicit text;
- optional reason.

Example:

```text
Degraded — high memory usage
Down — endpoint failed for 3 minutes
Unknown — no recent Docker metrics
```

### 29.8 Motion

Only functional micro-interactions:

- metric value transitions;
- subtle incoming chart point;
- resource status transition;
- panel expansion;
- incident appearance;
- restrained live indicator pulse.

Respect `prefers-reduced-motion` globally.

Do not use continuous decorative animation.

---

## 30. Chart Behavior

### 30.1 Default time ranges

```text
1 hour
6 hours
24 hours
7 days
30 days
Custom
```

### 30.2 Default chart information

Each metric chart should show:

- clear metric title;
- current value;
- minimum;
- average;
- maximum;
- selected time range;
- event/deployment annotations;
- explicit data gaps.

### 30.3 Progressive interaction

Default charts remain simple.

Expanded charts may support:

- hover/focus inspection;
- zoom;
- range selection;
- comparison against previous period later;
- multiple series where useful;
- raw/rollup resolution information for diagnostics.

### 30.4 Missing data

Rules:

```text
No sample: break the line
Collector unavailable: annotate or shade the gap
Resource intentionally stopped: show inactive interval
Persistence failure: mark history gap
```

Never replace missing data with zero unless zero was measured.

### 30.5 CPU semantics

Default container/resource CPU chart uses host-normalized percentage.

Detailed views may toggle to:

- host-normalized percentage;
- Docker-style percentage;
- CPU core equivalents.

The axis and tooltip must explicitly name the convention.

### 30.6 Performance

Charts must update incrementally. Do not destroy and recreate chart instances every 2 seconds.

Historical chart payloads should remain under approximately 1000 points per series by backend resolution selection.

---

## 31. Accessibility and Localization

### 31.1 Accessibility target

Target **WCAG 2.2 AA**.

Required:

- keyboard navigation;
- visible focus indicators;
- semantic HTML;
- correctly labeled controls;
- accessible dialog/menu behavior;
- sufficient contrast;
- reduced motion support;
- status not conveyed by color alone;
- screen-reader summaries for charts and key status panels.

Example chart summary:

> CPU averaged 21%, peaked at 78%, and is currently 18% for the selected 24-hour range.

### 31.2 Localization strategy

Ship English first, but the frontend must be localization-ready:

- strings stored in translation resources;
- stable message keys;
- locale-aware number formatting;
- locale-aware date/time formatting;
- locale-aware units and durations;
- no concatenated translated fragments;
- document community translation workflow.

Right-to-left layout support is not required in the initial alpha, but the architecture should not hard-code English strings inside components.

---

## 32. Events, Alerts, Incidents, and Checks

This section defines the intended post-alpha feature set. These features are not in `v0.1.0-alpha.1` unless explicitly moved by a later decision.

### 32.1 Alert rule philosophy

Use deterministic rules, not opaque anomaly detection, for operational alerts.

Default rule families:

```text
host CPU high
host memory high
filesystem usage high
inode usage high
container restart rate high
OOM event
health-check failure
collector degradation
database persistence failure
```

Users may override globally or per resource.

### 32.2 Rule fields

Each rule supports:

```text
threshold
comparison operator
trigger duration
recovery duration
severity
enabled/disabled
cooldown
maximum repeat frequency
notification targets
scope
maintenance suppression
```

Example:

```text
Memory usage > 85% for 10 minutes
Resolve after memory < 80% for 2 minutes
Repeat at most every 2 hours
Severity: Warning
```

### 32.3 Alert severities

Operational severities:

```text
Warning
Critical
```

Non-alert events:

```text
Info
```

### 32.4 Noise control

Required:

- trigger duration;
- recovery duration;
- cooldown;
- deduplication key;
- maximum repeat frequency;
- timed silences;
- deployment-aware grace periods.

### 32.5 Timed silences

Presets:

```text
30 minutes
1 hour
4 hours
until tomorrow
custom end time
```

Scopes:

```text
one resource
one alert rule
one project
the entire server
```

Recurring maintenance windows can be post-v1.

### 32.6 Deployment-aware grace period

When a confirmed or likely deployment begins:

- mark resource `Deploying`;
- suppress expected restart alerts;
- apply configurable health-check grace period;
- continue collecting all metrics/events;
- open incident if resource fails to recover.

Default grace period:

```text
5 minutes
```

### 32.7 Health checks

Auto-discover candidate domains from Coolify metadata when enrichment is available, but require user confirmation before enabling checks.

Default suggestion:

```text
Method: GET
Path: /
Expected status: 200–399
Interval: 30 seconds
Timeout: 5 seconds
```

User may change:

- route;
- method where supported;
- expected status/range;
- timeout;
- response-body match;
- enabled state.

Additional check types later:

- TCP port;
- TLS certificate expiry.

### 32.8 Health states

Deterministic model:

```text
Healthy: all required checks pass and resource signals normal
Degraded: warning threshold or optional check failure
Down: required endpoint down or resource unavailable
Paused: intentionally paused/stopped
Unknown: insufficient or stale data
```

### 32.9 Incidents

Incidents group related events over time.

Core fields:

```text
resource
started_at
resolved_at
severity
trigger
status
related events
```

Do not add assignments, comments, postmortems, escalation policies, or on-call schedules initially.

### 32.10 Automatic incident grouping example

```text
14:02 endpoint failed
14:03 container restarted
14:04 memory returned to normal
14:05 endpoint healthy
```

These may become one incident timeline when correlation rules indicate they belong together.

---

## 33. Notification System

Post-alpha notification channels:

```text
SMTP email
generic webhook
Discord webhook
Slack
Microsoft Teams
Telegram
```

All adapters implement one internal contract:

```text
ValidateConfiguration
SendTestMessage
SendAlert
SendRecovery
ReportDeliveryError
```

Provider-specific payload generation stays isolated from alert logic.

Notification secrets use the secret storage model defined earlier.

Retries must be bounded and must not block metric collection.

---

## 34. Logs

Logs are post-alpha.

### 34.1 Philosophy

Binnacle provides on-demand, read-only Docker log access. It does not become a persistent log indexing platform.

### 34.2 Retrieval options

Support:

```text
last 5 minutes
last 30 minutes
last hour
custom start time
tail N lines
follow live
```

Default caps:

```text
max lines: 5000
max response: 1 MiB
```

These are configurable within safe limits.

### 34.3 Search scope

Support:

- per-container search;
- per-logical-resource search across its components.

Whole-server search is deferred unless it can be implemented cheaply and bounded.

### 34.4 Severity highlighting

Heuristically classify common levels:

```text
ERROR
WARN
INFO
DEBUG
fatal
panic
```

Requirements:

- preserve raw line content;
- never discard unmatched lines;
- allow disabling highlighting;
- detect simple JSON log levels when trivial;
- do not create a full query language.

### 34.5 Time correlation

From a chart or event time, users should be able to view:

```text
Events within ±5 minutes
Container logs within ±5 minutes
Deployment annotations
Health transitions
Restart/OOM events
```

This is a key diagnostic feature.

### 34.6 Redaction

Best-effort server-side redaction before logs reach the browser.

Built-in patterns should target obvious secrets such as:

- Bearer authorization values;
- common API key formats;
- `password=...`;
- `secret=...`;
- `token=...`;
- private key blocks.

Support custom regex rules.

Redaction is not a security guarantee and must be labeled as best-effort.

Binnacle must not persist application logs in SQLite by default.

---

## 35. Diagnostics Bundle

The diagnostics feature should be preview-first.

Flow:

```text
Generate diagnostics
→ show exact included fields
→ user reviews
→ download bundle
```

Default included:

- Binnacle version and commit;
- OS and architecture;
- database schema version;
- collector health;
- sanitized configuration;
- recent internal errors;
- Docker API version;
- resource counts;
- database size;
- self-metrics summary.

Excluded by default:

- passwords;
- tokens;
- webhook URLs with secrets;
- full domains;
- IP addresses where unnecessary;
- container environment variables;
- application logs;
- database contents.

---

## 36. Data Export and Interoperability

Post-alpha lightweight export:

### CSV

Export selected metric data by:

- resource/server;
- metric;
- date range.

### JSON

Export:

- events;
- incidents;
- resource metadata.

### SQLite

Document direct access to the persistent database volume. Because SQLite may be in WAL mode, documentation must explain how to create a consistent copy rather than copying only `binnacle.db` while live.

### Prometheus-compatible endpoint

Post-alpha optional `/metrics`, disabled by default.

Expose stable, bounded-cardinality metrics:

- host CPU/memory/disk/network;
- resource CPU/memory;
- health-check status;
- collector health;
- Binnacle self-metrics.

Prometheus compatibility is an interoperability feature only. Binnacle internal storage and data model must not be redesigned around Prometheus labels.

---

## 37. Demo Mode

### 37.1 Requirement

Binnacle must provide synthetic demo mode:

```text
binnacle --demo
```

### 37.2 Demo behavior

Demo mode:

- requires no Docker socket;
- requires no host `/proc` mount;
- generates realistic host metrics;
- generates realistic container/resource metrics;
- simulates Coolify projects and services;
- simulates deployments;
- simulates restarts and OOMs;
- supports incident/check/alert scenarios when those modules exist;
- contains no real server data.

### 37.3 Uses

- public demo;
- frontend development;
- visual regression testing;
- documentation screenshots;
- accessibility testing;
- contributor onboarding;
- deterministic incident walkthroughs.

### 37.4 Determinism

Support a seed or scenario selection so tests can reproduce demo states.

Example:

```text
binnacle --demo --demo-seed 42
```

---

## 38. Self-Observation

Binnacle must measure itself from v1.

Expose under:

```text
Settings → System → Monitor health
```

Required metrics:

- Binnacle CPU usage;
- Binnacle RSS/working memory;
- Go heap metrics where useful;
- goroutine count;
- SQLite database file size;
- WAL size;
- persistence write latency;
- rollup duration;
- retention duration;
- samples collected per second;
- collector duration;
- dropped sample/batch count;
- persistence queue depth;
- active SSE clients;
- Docker API request duration/error counts;
- internal collector state.

These metrics should be available in diagnostics and optional Prometheus export later.

---

## 39. Performance Requirements

### 39.1 Core targets

For one host with up to 30 running containers:

```text
Steady/idle memory target:       < 50 MB RSS
Investigation threshold:         > 80 MB RSS
Average CPU target:              < 0.5% of one CPU core
Persistence write p95:           < 50 ms
Idle SSE traffic/client:         < 10 KB/s
```

These are benchmark targets, not universal guarantees.

### 39.2 Performance engineering rules

Implementers MUST follow these hot-path principles:

1. Reuse buffers where practical.
2. Avoid repeated JSON serialization for identical live snapshots per client; serialize once and fan out.
3. Avoid repeated metadata inspection.
4. Subscribe to Docker events instead of polling static metadata constantly.
5. Keep live state in memory.
6. Batch SQLite writes in transactions.
7. Use prepared statements where appropriate.
8. Keep queues bounded.
9. Avoid per-sample regex work.
10. Avoid full process scans unless requested.
11. Avoid chart API payloads with excessive points.
12. Avoid one goroutine per metric.
13. Profile before micro-optimizing.
14. Track allocations in benchmark suites for collectors and serialization.

### 39.3 Benchmark matrix

Publish reproducible benchmark results for:

```text
1 host / 10 containers
1 host / 30 containers
1 host / 50 containers
1 host / 100 containers
```

Measure:

- RSS over at least 30 minutes;
- average and p95 CPU usage;
- Docker API request rate;
- SQLite writes/sec;
- persistence latency;
- database growth/day;
- SSE bandwidth/client;
- collection cycle duration;
- allocations/sample where practical.

### 39.4 Validation against reference tools

Alpha release must validate metrics against Linux and Docker reference outputs during integration tests/manual release qualification:

- host CPU/memory against standard Linux interfaces/tools;
- Docker CPU/memory/network against Docker stats within documented semantic differences;
- filesystem usage against `statfs`/`df` semantics;
- rate calculations against fixture counters.

---

## 40. Resource Safeguards

Binnacle must protect the host from Binnacle itself.

Required safeguards:

- minimum collection interval of 1 second;
- bounded persistence queue;
- bounded SSE client buffers;
- maximum chart points per series;
- Docker API concurrency cap;
- health-check concurrency cap later;
- log response/line caps later;
- bounded retry queues;
- database budget thresholds;
- cancellation timeouts for external requests.

Recommended Coolify memory limit in documentation:

```yaml
deploy:
  resources:
    limits:
      memory: 128M
```

This is a recommended deployment guardrail, not an internal hard-coded application ceiling.

The product should continue to work on installations that legitimately need more memory.

---

## 41. Collector Health and Failure Semantics

### 41.1 Independent collector states

Maintain independent health for:

```text
Host collector
Docker collector
Coolify enrichment
Storage writer
Rollup worker
Retention worker
Health-check engine later
Notification delivery later
```

One degraded integration must not mark the whole server down.

### 41.2 Stateful degradation

Recommended transition logic:

```text
Single transient failure:
- record debug/internal trace
- retry
- do not alert user immediately

Repeated failures:
- collector state = Degraded
- show reason

Sustained failure:
- collector state = Down
- create event/incident later
- notify later if configured

Recovery:
- state = Healthy
- record duration and recovery event
```

Exact failure thresholds should be per collector and configurable internally; default behavior should avoid alerting on one temporary Docker API timeout.

### 41.3 Freshness

Every collector domain tracks `last_success_at`.

The UI must show stale data as stale/unknown, not current.

---

## 42. Manual Data Deletion

Support scoped destructive operations:

```text
Delete history for one resource
Delete data older than a chosen date
Reset all monitoring history
Purge archived resource
```

Requirements:

- show exact scope preview;
- require typed confirmation for destructive actions;
- explain irreversibility;
- do not alter monitoring configuration unless explicitly selected;
- run large deletions in bounded batches;
- keep UI informed of progress.

No built-in backup is required in v1.

---

## 43. API and Internal Extension Boundaries

Before 1.0, define internal interfaces without promising external plugin stability.

Required boundaries:

```text
HostCollector
ContainerRuntimeCollector
ResourceResolver
CoolifyEnricher
NotificationAdapter
HealthCheckType
StorageRepository
AuthProvider
DataExporter
```

Do not implement a Go plugin system.

Do not execute arbitrary community scripts.

New integrations should initially arrive as reviewed pull requests.

A future external extension protocol may use a constrained HTTP or executable protocol, but only after real community demand.

---

## 44. Build and Developer Experience

### 44.1 Primary commands

Required Make targets:

```text
make dev
make dev-demo
make dev-host
make test
make check
make build
```

Expected behavior:

#### `make dev`

Start:

- Go backend with reload;
- Svelte dev server;
- synthetic demo collector;
- temporary development SQLite database.

#### `make dev-demo`

Run deterministic synthetic data mode.

#### `make dev-host`

Run against real local host/Docker interfaces with clear permission requirements.

#### `make test`

Run unit/integration test suite appropriate for local environment.

#### `make check`

Run most CI quality gates locally.

#### `make build`

Build production frontend, embed assets, and produce Binnacle binary/image artifacts.

### 44.2 Package manager

Use pnpm with committed lockfile.

The root project should declare the package manager version or use Corepack-compatible metadata.

Use the active Node LTS at project initialization and pin it in development/CI metadata.

### 44.3 Development container

Provide a devcontainer or equivalent containerized development option for contributors who do not want to install Go, Node, pnpm, and native build dependencies locally.

Native development remains the fastest path.

---

## 45. Testing Strategy

### 45.1 Required quality gates

CI must include:

- Go formatting;
- Go vet;
- static analysis;
- Go unit tests;
- race-sensitive tests where practical;
- SQLite migration tests;
- rollup correctness tests;
- retention tests;
- Svelte/TypeScript checking;
- frontend linting;
- frontend unit tests;
- accessibility smoke tests;
- API contract tests;
- container build;
- dependency vulnerability scan;
- license compatibility checks.

Do not enforce arbitrary code coverage percentages. Require meaningful behavioral tests for regression-prone logic.

### 45.2 Fixture-driven collector tests

Maintain fixtures for:

- `/proc/stat`;
- `/proc/meminfo`;
- `/proc/net/dev`;
- `/proc/diskstats`;
- cgroup v2 files;
- Docker stats payloads;
- Docker inspect payloads;
- Docker lifecycle event streams;
- Compose labels;
- Coolify metadata samples across supported versions when available.

### 45.3 Metrics formula tests

Must test:

- CPU delta math;
- counter reset behavior;
- network rate calculation;
- Docker CPU normalization;
- cgroup CPU conversion;
- memory working-set calculation;
- resource aggregation during overlapping deployments;
- missing data propagation.

### 45.4 Storage tests

Must test:

- fresh migration sequence;
- migration from every released alpha schema;
- idempotent rollups;
- partial bucket handling;
- retention cutoffs;
- database busy retry behavior;
- queue overflow behavior;
- disk budget state transitions;
- SQLite WAL-aware export/copy strategy where implemented.

### 45.5 Frontend tests

Must cover:

- first-run onboarding;
- login/logout;
- theme switching;
- density switching;
- overview rendering;
- resource detail rendering;
- explicit chart gaps;
- collector degradation UI;
- settings source labels;
- keyboard navigation smoke path;
- reduced-motion behavior;
- mobile overview smoke path.

---

## 46. Security Engineering

### 46.1 Permanent read-only rule

The product must remain read-only with respect to monitored workloads.

Prohibited features:

```text
restart container
stop container
start container
delete container
exec command
open shell
change Docker configuration
redeploy Coolify resource
mutate application environment
```

If any future proposal attempts to add these, it requires an explicit product decision and security review. The default assumption is rejection.

### 46.2 CSRF

Any state-changing browser endpoint such as settings changes or session operations must use appropriate CSRF defenses in addition to SameSite cookies.

### 46.3 Rate limiting

At minimum rate-limit:

- login attempts;
- setup token attempts;
- diagnostics generation;
- expensive metric queries;
- log retrieval later.

### 46.4 Input limits

Set explicit limits for:

- JSON request body size;
- query range;
- number of requested metrics;
- regex length later;
- custom redaction patterns later;
- health-check response body sampling later.

### 46.5 Release security artifacts

From first public release:

- `SECURITY.md`;
- supported-version policy;
- private vulnerability reporting instructions;
- dependency update policy;
- software bill of materials;
- signed release artifacts where feasible;
- provenance/attestations where feasible;
- reproducible or near-reproducible build practices.

### 46.6 Diagnostics privacy

Never include secrets or application logs by default.

### 46.7 Container hardening

The production container should:

- run as non-root where host access permits;
- use read-only root filesystem;
- use a writable persistent data volume only where needed;
- set `no-new-privileges` when deployment platform supports it;
- avoid `privileged: true`;
- avoid unnecessary Linux capabilities;
- include a minimal base image;
- expose only the application HTTP port.

If socket permissions force group mapping, document the security implications clearly.

---

## 47. Update and Migration Behavior

### 47.1 Versioning

Use semantic prerelease versions:

```text
v0.1.0-alpha.1
v0.1.0-alpha.2
v0.1.0-beta.1
v0.1.0
```

### 47.2 Compatibility phases

`0.x`:

- deliberate incubation;
- configuration may change;
- database schemas may evolve;
- APIs remain experimental unless explicitly stabilized;
- automatic forward migrations required;
- downgrade not guaranteed unless documented.

`1.0+`:

- semantic versioning;
- in-place upgrades;
- automatic forward migrations;
- stable configuration keys;
- documented backup/restore path;
- stable agent/server protocol within a major version once agents exist;
- compatibility policy for at least one previous minor agent version later.

### 47.3 Migration process

Before migration:

1. check database integrity;
2. check available disk space;
3. record current schema version;
4. apply migration in a transaction where SQLite supports it safely;
5. verify resulting schema;
6. only then continue application startup.

Because v1 has no built-in backups, avoid destructive migrations whenever possible.

Potentially destructive migrations must either:

- require explicit confirmation; or
- create a temporary database copy if sufficient space exists.

### 47.4 Migration failure

On failure:

- do not repeatedly mutate the DB in a crash loop;
- preserve clear migration error logs;
- expose recovery guidance where possible;
- do not automatically discard data.

### 47.5 Update UX

Coolify deployment:

```text
New version available
View release notes
Open resource in Coolify
Copy recommended image tag
```

Native deployment later:

```text
New version available
Copy package upgrade command
```

No self-update in v1.

---

## 48. Public Website and Documentation

Initial product site:

```text
Home
Demo / screenshots
Install
Documentation
Security
Contributing
GitHub
```

Primary headline:

> **Lightweight, Coolify-aware monitoring for Docker servers.**

Supporting message:

> Host metrics, container history, deployments, health checks, incidents, and logs—without Prometheus, Grafana, or external telemetry.

The website should emphasize:

- host + container monitoring;
- Coolify-aware grouping;
- built-in history;
- read-only design;
- no telemetry;
- local data;
- simple installation;
- AGPL source availability;
- measured resource usage.

Comparison pages may be added after alpha works. Comparisons with Glances, Netdata, and Grafana must be factual, narrow, and kept current.

Stable comparison dimensions:

- built-in historical storage;
- Coolify grouping;
- installation dependencies;
- external telemetry;
- container logs;
- health checks;
- read-only design;
- resource footprint;
- supported platforms.

Avoid unsupported superiority claims.

---

## 49. Branding and Naming

### 49.1 Product name

The permanent product name is **Binnacle**.

```text
Repository: binnacle
Binary: binnacle
Container: ghcr.io/drilonrecica/binnacle
UI: Binnacle
```

README and user-facing interfaces must use `Binnacle`; technical identifiers use `binnacle`, and environment variables use the `BINNACLE_` prefix.

### 49.2 Naming rationale

The Binnacle name is:

- technical but ownable;
- signal/clarity oriented;
- one word;
- easy to pronounce internationally;
- ideally 5–9 letters;
- not locked to Docker, VPS, host, or server;
- suitable as a CLI binary and repository name.

### 49.3 Visual mark

Use a hybrid abstract telemetry symbol.

Potential directions:

- waveform forming a subtle monogram;
- metric bars arranged into a unique glyph;
- signal trace crossing a node/grid;
- asymmetric live-state ring;
- terminal cursor transformed into a telemetry symbol.

Avoid:

- generic heartbeat line;
- Docker whale references;
- shields;
- meaningless hexagons;
- server-rack clip art.

The mark must work at favicon size, GitHub avatar size, sidebar size, and large lockup size.

### 49.4 Product voice

Voice:

- calm;
- concise;
- factual;
- non-alarmist;
- never cute during incidents;
- technical detail available on demand.

Good:

> History recording paused because the database volume is full.

Bad:

> Uh-oh! Your database is having a bad day 😅

Primary message plus expandable technical detail is the preferred error pattern.

---

## 50. Alpha Scope

### 50.1 Included in `v0.1.0-alpha.1`

Required:

- secure first-run admin setup;
- host CPU metrics;
- host memory and swap;
- load averages;
- disk I/O;
- filesystem usage and inode usage;
- host network metrics;
- Docker container discovery;
- Docker container CPU/memory/network/block I/O/PID metrics;
- Docker lifecycle events;
- OOM events;
- basic Coolify/Compose grouping;
- stable logical resource identity across replacements;
- multi-container resource aggregation;
- live current-state dashboard via SSE;
- SQLite historical storage;
- typed schema;
- automatic rollups;
- retention presets and advanced overrides;
- database budget safeguards;
- historical charts;
- explicit chart gaps;
- resource detail pages;
- archived resource history;
- events page;
- collector health;
- dark/light/system themes;
- comfortable/compact density modes;
- responsive mobile overview;
- settings dashboard;
- self-monitoring metrics;
- Docker Compose installation;
- Coolify installation template;
- synthetic demo mode;
- basic diagnostics view;
- install/update/uninstall/recovery documentation.

### 50.2 Explicitly excluded from alpha.1

- notifications;
- automatic incident grouping;
- HTTP health checks;
- TLS checks;
- container log viewer;
- process explorer;
- Coolify API enrichment;
- TOTP;
- external authentication;
- Prometheus endpoint;
- public API tokens;
- CSV/JSON export;
- custom dashboard personalization;
- multi-server agents;
- database-specific integrations;
- eBPF features;
- packet inspection.

These exclusions are deliberate scope control.

---

## 51. Alpha Release Gates

`v0.1.0-alpha.1` must not be published until:

- no known critical security issue remains;
- no known normal-operation data-loss bug remains;
- fresh database migration sequence passes;
- fresh Coolify install is tested;
- Docker Compose install is tested;
- upgrade path is tested once a prior alpha exists;
- host metrics are validated;
- Docker metrics are validated;
- resource identity survives redeployment in test fixtures;
- overlapping deployment aggregation is tested;
- retention and rollups are tested;
- persistence failure does not kill live monitoring;
- memory target is benchmarked;
- CPU overhead is benchmarked;
- SSE traffic is measured;
- keyboard navigation smoke test passes;
- dark and light themes are reviewed;
- mobile overview smoke test passes;
- install docs exist;
- update docs exist;
- uninstall docs exist;
- recovery docs exist;
- Docker socket security warning is documented;
- AGPL license and security policy are included.

Small documented visual defects may remain.

---

## 52. Implementation Roadmap

### Phase 0 — Repository and architecture foundation

Deliverables:

- monorepo structure;
- Go application bootstrap;
- Svelte 5 app bootstrap;
- pnpm lockfile;
- Vite production build;
- Go asset embedding;
- root context and graceful shutdown;
- config loader with precedence;
- logging conventions;
- SQLite connection/migration framework;
- ADRs 001–014;
- Make targets;
- CI skeleton.

Acceptance criteria:

- `make dev-demo` launches backend + Svelte UI;
- `make build` produces one runnable binary/image;
- no external CDN required;
- database migrations run on fresh install.

### Phase 1 — Demo mode and UI shell

Deliverables:

- deterministic synthetic host/resource generator;
- overview shell;
- navigation;
- theme system;
- density modes;
- basic responsive layout;
- SSE plumbing;
- current-state store in Svelte runes;
- synthetic events.

Acceptance criteria:

- UI can be developed without Docker;
- demo scenario is deterministic with seed;
- live updates occur every 2 seconds;
- no full-page chart recreation.

### Phase 2 — Host collector

Deliverables:

- `/proc/stat` CPU collector;
- memory collector;
- load/uptime collector;
- network collector;
- diskstats collector;
- filesystem collector;
- boot identity/reboot event handling;
- fixture tests;
- host current-state API;
- server detail page.

Acceptance criteria:

- metrics match reference outputs within documented semantics;
- collector survives missing optional files;
- counter resets create gaps/reset boundaries, not negative rates.

### Phase 3 — Docker collector

Deliverables:

- Docker API client wrapper with read-only interface;
- startup discovery;
- event subscription;
- metadata cache;
- stats collection;
- CPU normalization;
- memory working set;
- network/block I/O rates;
- PID count;
- health/status metadata;
- lifecycle events;
- OOM detection;
- reconciliation loop.

Acceptance criteria:

- no `docker` CLI process spawning;
- static metadata is not inspected every 2 seconds;
- metrics validate against Docker stats;
- Docker outage degrades independently.

### Phase 4 — Resource resolver and Coolify grouping

Deliverables:

- logical resource model;
- Compose label resolver;
- Coolify-aware resolver fixtures;
- stable identity fallback chain;
- category inference;
- user category override storage;
- multi-container aggregation;
- Coolify infrastructure grouping;
- unmanaged container view;
- archived resources.

Acceptance criteria:

- redeploying a container does not break logical history;
- overlapping old/new instances aggregate correctly;
- user category overrides survive replacement.

### Phase 5 — Metrics Engine and persistence

Deliverables:

- central Metrics Engine;
- current-state cache;
- immutable snapshot publishing;
- persistence batch creation;
- bounded queue;
- single SQLite writer;
- raw sample tables;
- event tables;
- collector state tables;
- degraded persistence behavior.

Acceptance criteria:

- live state continues when SQLite writes are forced to fail;
- queue overflow drops oldest batch and increments counter;
- one live snapshot serialization can be shared across clients.

### Phase 6 — Historical queries, rollups, retention

Deliverables:

- 1m/15m/1h rollups;
- idempotent bucket processing;
- tier cleanup;
- retention presets;
- advanced retention settings;
- DB budget monitoring;
- emergency raw-persistence pause;
- automatic resolution selection;
- historical metrics endpoint.

Acceptance criteria:

- no zero-filling of missing data;
- chart point count bounded;
- expired raw data not deleted before rollup exists;
- DB budget state visible in UI.

### Phase 7 — Authentication and onboarding

Deliverables:

- admin user schema;
- Argon2id password hashing;
- session tokens and hashed storage;
- rate limiting;
- one-time setup token;
- browser setup;
- environment/secret bootstrap;
- guided diagnostics;
- setup checklist.

Acceptance criteria:

- arbitrary visitor cannot claim fresh public install without setup token;
- setup mode disables after admin creation;
- session rotation works;
- logout-all invalidates sessions.

### Phase 8 — Product UI completion

Deliverables:

- final overview;
- resource lists;
- resource detail;
- charts;
- events page;
- server detail;
- settings pages;
- self-monitoring page;
- explicit missing-data UI;
- collector degradation UI;
- archived resources UI;
- destructive data deletion flows.

Acceptance criteria:

- first screen answers health/attention questions;
- compact and comfortable modes work;
- charts are accessible and responsive;
- mobile overview is usable.

### Phase 9 — Packaging, docs, and release qualification

Deliverables:

- production Dockerfile;
- GHCR workflow;
- Compose file;
- Coolify template;
- update channel metadata;
- install docs;
- update docs;
- uninstall docs;
- recovery docs;
- Docker socket hardening docs;
- benchmark suite;
- benchmark report;
- SBOM/signing pipeline where feasible;
- security policy;
- release checklist.

Acceptance criteria:

- fresh Coolify deploy succeeds from documented path;
- stable tag policy enforced;
- alpha gates pass.

---

## 53. Post-Alpha Roadmap

Order may change based on user feedback, but intended sequence:

### v0.2 — Checks and alerts

- HTTP health checks;
- deterministic health model;
- basic alert rules;
- warning/critical severity;
- cooldown/dedup/recovery;
- timed silences;
- deployment grace period;
- alert UI.

### v0.3 — Notifications and incidents

- SMTP;
- generic webhook;
- Discord;
- Slack;
- Microsoft Teams;
- Telegram;
- lightweight incident grouping;
- incident timeline.

### v0.4 — Logs and process diagnostics

- bounded Docker logs;
- follow stream;
- per-resource search;
- severity highlighting;
- server-side redaction;
- time-linked correlation;
- read-only process explorer.

### v0.5 — Coolify enrichment and external auth

- optional read-only Coolify API;
- richer project/environment/domain metadata;
- better deployment confirmation;
- TOTP;
- trusted proxy/external auth.

### v0.6 — Interoperability

- read-only API tokens;
- CSV/JSON export;
- optional Prometheus endpoint;
- limited personalization.

### Later

- optional database integrations (PostgreSQL, MySQL, Redis);
- central multi-server dashboard;
- lightweight outbound-only agents;
- optional additional runtime support only after demand;
- anomaly hints as suggestions, never opaque automatic outage decisions.

---

## 54. Internal Interface Sketches

These are conceptual contracts to guide package design. Exact Go syntax may evolve, but responsibility boundaries must remain.

### 54.1 Host collector

```go
type HostCollector interface {
    Collect(ctx context.Context, prev *HostCounterState) (HostObservation, HostCounterState, error)
    Health() CollectorHealth
}
```

Collector should separate raw counter state from normalized observation.

### 54.2 Docker collector

```go
type ContainerCollector interface {
    Reconcile(ctx context.Context) ([]ContainerMetadata, error)
    CollectStats(ctx context.Context, ids []string) ([]ContainerObservation, error)
    Events(ctx context.Context) (<-chan DockerEvent, <-chan error)
}
```

The rest of the codebase should not receive unrestricted Docker client access.

### 54.3 Resource resolver

```go
type ResourceResolver interface {
    Resolve(meta ContainerMetadata) ResourceIdentityCandidate
    Group(all []ContainerMetadata) ResourceGraph
}
```

### 54.4 Metrics engine

```go
type MetricsEngine interface {
    IngestHost(obs HostObservation)
    IngestContainers(obs []ContainerObservation)
    IngestMetadata(update MetadataUpdate)
    IngestEvent(event NormalizedEvent)
    Snapshot() CurrentSnapshot
    Subscribe() SnapshotSubscription
}
```

Actual implementation may prefer channels/event loop to direct methods.

### 54.5 Storage writer

```go
type PersistenceWriter interface {
    WriteBatch(ctx context.Context, batch PersistenceBatch) error
}
```

### 54.6 Historical query service

```go
type MetricsQueryService interface {
    Query(ctx context.Context, q MetricsQuery) (MetricsSeriesResponse, error)
}
```

It owns automatic tier selection and point limits.

---

## 55. API Payload Sketches

### 55.1 Server summary

```json
{
  "id": "srv_01...",
  "hostname": "my-vps",
  "status": "healthy",
  "uptimeSeconds": 913244,
  "cpuCount": 4,
  "cpu": {
    "busyPct": 8.2,
    "iowaitPct": 0.3,
    "stealPct": 0.0
  },
  "memory": {
    "totalBytes": 8126070784,
    "usedBytes": 2684354560,
    "availableBytes": 5441716224,
    "usedPct": 33.0
  },
  "load": {
    "one": 0.42,
    "five": 0.37,
    "fifteen": 0.31
  },
  "freshness": {
    "host": "2026-07-11T12:00:00Z",
    "docker": "2026-07-11T12:00:00Z"
  }
}
```

### 55.2 Resource list item

```json
{
  "id": "res_01...",
  "name": "Directus production",
  "project": "customer-portal",
  "environment": "production",
  "category": "service",
  "status": "healthy",
  "archived": false,
  "cpuHostPct": 7.1,
  "memoryWorkingSetBytes": 859832320,
  "networkRxBps": 17540,
  "networkTxBps": 8320,
  "activeInstanceCount": 4,
  "lastSeenAt": "2026-07-11T12:00:00Z"
}
```

### 55.3 Metrics series

```json
{
  "scope": "resource",
  "id": "res_01...",
  "from": "2026-07-10T12:00:00Z",
  "to": "2026-07-11T12:00:00Z",
  "resolution": "1m",
  "series": [
    {
      "metric": "cpu_host_pct",
      "unit": "percent_of_host",
      "points": [
        {"ts": "2026-07-11T11:58:00Z", "min": 1.2, "avg": 2.4, "max": 8.7, "count": 6},
        {"ts": "2026-07-11T11:59:00Z", "min": 1.0, "avg": 1.8, "max": 3.2, "count": 6}
      ]
    }
  ],
  "gaps": [
    {
      "from": "2026-07-11T10:30:00Z",
      "to": "2026-07-11T10:35:00Z",
      "reason": "docker_collector_unavailable"
    }
  ]
}
```

### 55.4 Event

```json
{
  "id": "evt_01...",
  "ts": "2026-07-11T11:03:21Z",
  "severity": "info",
  "type": "container_replaced",
  "summary": "API container replaced during likely deployment",
  "resourceId": "res_01...",
  "containerInstanceId": "abc123...",
  "source": "docker",
  "details": {
    "deploymentConfidence": "likely"
  }
}
```

---

## 56. User Experience Acceptance Scenarios

### Scenario A — Fresh Coolify install

1. User adds Binnacle service template in Coolify.
2. User assigns a domain and deploys.
3. User opens Binnacle.
4. Setup token is required.
5. User creates admin credentials.
6. Binnacle verifies `/proc`, `/sys`, Docker socket, and storage.
7. Binnacle detects Compose/Coolify resources.
8. User accepts Balanced retention preset.
9. Dashboard opens with live host and resource metrics.

Success means no terminal commands are required for the normal Coolify path.

### Scenario B — Docker API outage

1. Docker API becomes unavailable.
2. Host metrics continue.
3. Docker collector becomes Degraded, then Down if sustained.
4. Resource data freshness visibly ages and becomes Unknown.
5. Binnacle does not show stale resource values as current.
6. When Docker returns, collector recovers and event is recorded.

### Scenario C — SQLite disk full

1. Persistence write fails.
2. Live dashboard continues.
3. Writer retries with backoff.
4. Batches queue up to limit.
5. Oldest batches drop after overflow.
6. `History persistence degraded` warning appears.
7. Self-metrics show queue depth and dropped batches.
8. When storage recovers, writes resume.

### Scenario D — Coolify redeploy

1. Existing app container is running.
2. New container starts.
3. Both overlap for 30 seconds.
4. Resource CPU/RAM shows real combined usage.
5. New container becomes active.
6. Old container stops.
7. Resource history remains continuous under one logical resource ID.
8. Deployment/replacement annotation appears.

### Scenario E — Archived resource

1. Application is removed from Coolify.
2. After reconciliation confirms disappearance, resource becomes Archived.
3. It disappears from default overview.
4. Historical charts remain accessible under Archived.
5. User may purge history with typed confirmation.

### Scenario F — Mobile check during incident

On phone, user can immediately see:

- server health;
- unhealthy resources;
- CPU/RAM/disk summary;
- recent critical events;
- compact resource chart.

Dense process/log tables remain secondary.

---

## 57. Open Risks and Mitigations

### Risk: Docker socket privilege

**Mitigation:** permanent read-only product behavior, narrow internal Docker client interface, no arbitrary proxying, hardened socket proxy recommendation.

### Risk: Coolify metadata changes

**Mitigation:** resolver isolation, fixture tests, fallback to Compose metadata, optional API enrichment, no dependency on undocumented Coolify DB internals.

### Risk: SQLite growth

**Mitigation:** tiered retention, automatic rollups, typed tables, budget warnings, emergency raw persistence pause, predictable query resolution.

### Risk: Go memory growth

**Mitigation:** self-metrics, benchmark matrix, bounded buffers, careful serialization, metadata caching, pprof in development/diagnostics only where safely exposed.

### Risk: high Docker stats overhead with many containers

**Mitigation:** concurrency cap, benchmark at 10/30/50/100 containers, event-driven metadata cache, optional adaptive collection strategy only if benchmarks justify it.

### Risk: scope creep

**Mitigation:** explicit alpha exclusions and non-goals. New major features require product decision and likely ADR/RFC.

### Risk: misleading charts from gaps

**Mitigation:** explicit missing values, line breaks, gap annotations, no zero substitution.

### Risk: first-run takeover on public URL

**Mitigation:** one-time high-entropy setup token and automatic setup-mode shutdown.

### Risk: secret leakage in diagnostics/logs

**Mitigation:** server-side redaction, preview-first diagnostics, no app logs by default, encrypted secret storage.

---

## 58. Final Decision Ledger

The following decisions are binding unless explicitly amended:

1. Build a lightweight self-hosted monitoring product, not a generic observability suite.
2. Primary audience is developers running Docker on one or more VPSs.
3. Use smart defaults with advanced settings.
4. Be Coolify-first but not Coolify-only.
5. Single-server first; multi-server architecture later.
6. Primary deployment is Docker/Coolify, not systemd.
7. Go backend, not Rust, for v1.
8. Svelte 5 with runes and TypeScript frontend.
9. SQLite local history storage.
10. Typed time-series tables, not generic metric-name/value schema.
11. Central Metrics Engine and in-memory live state.
12. SSE for live updates.
13. Historical API auto-selects resolution.
14. Tiered retention with configurable presets and advanced overrides.
15. Default Balanced retention: 48h raw, 30d 1m, 1y 15m, 1h indefinite.
16. Default live collection every 2s; persistence every 10s.
17. Settings configurable in Binnacle UI, with declarative config support.
18. Config precedence: defaults < file < env < UI overrides for eligible settings.
19. Docker metadata cached and event-driven.
20. Host and every Docker container/resource monitored.
21. Logical resource identity survives redeployments.
22. Multi-container Coolify resources aggregate with expandable components.
23. Container CPU defaults to host-normalized percentage.
24. Missing data appears as gaps, never fabricated zero.
25. Read-only forever as product philosophy; no workload control in v1 or planned default.
26. One local admin in v1.
27. Argon2id + secure sessions + rate limits.
28. Hybrid secrets: env, Docker secrets, encrypted SQLite.
29. No telemetry by default.
30. Fully offline-capable core.
31. Dark, light, and system themes.
32. Comfortable and compact density modes.
33. Visual direction: modern telemetry console with a terminal soul.
34. Professional with personality; no generic SaaS template look.
35. WCAG 2.2 AA target.
36. English first, localization-ready architecture.
37. Monorepo.
38. pnpm frontend package manager.
39. Conventional Commits.
40. Lightweight founder-led governance with DCO and RFC/ADR for major changes.
41. AGPL-3.0-only.
42. Repository remains on owner's personal GitHub account.
43. Permanent product name Binnacle.
44. Repository, binary, container image, and technical namespace use `binnacle`.
45. No backups in v1.
46. Named persistent volume default in Coolify.
47. Release channels stable/beta/edge with immutable exact versions.
48. Coolify one-click template is a core distribution channel.
49. Basic alerts later with deterministic rules.
50. Notifications later: SMTP, generic webhook, Discord, Slack, Teams, Telegram.
51. Lightweight automatic incidents later.
52. Health-check discovery with user confirmation later.
53. Logs later: bounded, on-demand, not persisted/indexed.
54. Process explorer later: read-only, demand-driven.
55. Generic disk/I/O metrics first; database-specific integrations later.
56. Public read-only API tokens later.
57. Optional Prometheus endpoint later, disabled by default.
58. Limited dashboard personalization later, not freeform builder.
59. Synthetic demo mode required.
60. Self-monitoring required from v1.
61. Performance target <50 MB steady RSS for 30-container reference case.
62. Average CPU target <0.5% of one core for reference case.
63. SQLite write p95 target <50 ms.
64. Idle SSE traffic target <10 KB/s/client.
65. Alpha is a polished vertical slice, not every planned feature.

---

## 59. Glossary

**Host**  
The Linux VPS on which Binnacle runs and which it monitors.

**Container instance**  
One concrete Docker container ID. Ephemeral across redeployments.

**Logical resource**  
A stable Binnacle representation of a Coolify application, service, Compose service group, or unmanaged container grouping.

**Component**  
A container or service member inside a multi-container logical resource.

**Metrics Engine**  
The central in-memory normalization, aggregation, current-state, and fan-out layer.

**Raw sample**  
The high-resolution persisted metric record, default 10-second resolution.

**Rollup**  
A lower-resolution aggregate storing min/max/avg/count and p95 only where specified.

**Collector health**  
Independent health state of host, Docker, Coolify, storage, and other subsystems.

**Deployment confidence**  
Classification of a detected rollout as Confirmed, Likely, or Container replacement.

**History persistence degraded**  
State where live monitoring works but SQLite history writes are failing or dropping queued batches.

**Read-only**  
Binnacle may observe and configure itself, but may not mutate monitored Docker workloads or host operational state.

---

## 60. Definition of Done for the Specification

An implementation is conformant with this specification only when:

- the alpha scope is implemented without silently adding excluded major features;
- the performance and graceful-degradation principles are preserved;
- live metrics do not depend on SQLite reads;
- Docker workload operations remain read-only;
- Coolify logical resource identity survives container replacement;
- historical storage uses typed SQLite schemas and tiered retention;
- the UI implements the agreed information architecture and design direction;
- user-configurable sampling and retention are available in Settings;
- no telemetry is sent by default;
- self-observation is visible;
- installation through Coolify is a first-class tested path;
- security, accessibility, release, and benchmark gates are met.

This document intentionally leaves room for low-level implementation choices only where those choices do not change product behavior, security boundaries, performance targets, data semantics, or user-visible contracts. Any change to those areas must be recorded as a deliberate decision rather than inferred silently during implementation.

---

## Appendix A — Suggested Alpha Backlog as Agentic Work Packages

The following work packages are deliberately small enough to be assigned to separate implementation agents while maintaining clear interfaces.

### A1. Repository bootstrap

- create repository tree;
- initialize Go module;
- initialize Svelte 5 app;
- configure pnpm;
- add Makefile;
- add CI skeleton;
- add license/governance/security documents.

### A2. Config loader

- define typed config struct;
- defaults;
- TOML file loading;
- environment mapping;
- UI override storage interface;
- effective-value/source reporting;
- validation.

### A3. Application lifecycle

- root context;
- structured logging;
- graceful shutdown;
- health endpoint;
- component start/stop orchestration.

### A4. SQLite bootstrap

- connection manager;
- pragmas;
- migrations table;
- migration runner;
- integrity preflight;
- schema version reporting.

### A5. Demo generator

- seeded pseudo-random generator;
- host metric scenario;
- resource scenario;
- deployment scenario;
- failure scenario;
- deterministic clock option for tests.

### A6. Metrics domain types

- units;
- nullable sample semantics;
- host observation;
- container observation;
- resource snapshot;
- event types;
- collector health state.

### A7. Metrics Engine

- ingest channels;
- current state;
- resource aggregation;
- snapshot sequence;
- subscriber fan-out;
- persistence batch scheduler;
- bounded queue.

### A8. SSE service

- authenticated stream;
- snapshot event;
- heartbeat;
- per-client bounded buffer;
- slow-client handling;
- serialization reuse.

### A9. Host CPU/memory collector

- parser fixtures;
- delta state;
- CPU formulas;
- memory formulas;
- load and uptime.

### A10. Host network/disk/filesystem collector

- counter parser;
- rate calculation;
- reset handling;
- device/interface classification;
- statfs integration;
- fixture tests.

### A11. Docker read-only client layer

- interface exposing only read methods;
- list/inspect/stats/events/version;
- error normalization;
- timeouts;
- concurrency limits.

### A12. Docker metadata cache

- startup discovery;
- event updates;
- periodic reconciliation;
- cache invalidation;
- metadata snapshots.

### A13. Docker stats normalizer

- CPU formula;
- memory working set;
- network rates;
- block I/O rates;
- PIDs;
- counter reset handling;
- tests.

### A14. Resource resolver

- Compose labels;
- Coolify fixtures;
- stable key generation;
- unmanaged fallback;
- category inference;
- override merge.

### A15. Resource aggregation

- current membership;
- sum metrics;
- overlap semantics;
- state rollup;
- archived lifecycle;
- tests.

### A16. Event normalizer

- Docker event mapping;
- OOM deduplication;
- replacement correlation;
- boot events;
- collector events.

### A17. Persistence writer

- typed insert statements;
- batched transaction;
- backoff;
- queue overflow metrics;
- degraded state.

### A18. Rollup engine

- 1m aggregation;
- 15m aggregation;
- 1h aggregation;
- idempotency;
- partial-bucket handling;
- tests.

### A19. Retention engine

- presets;
- cutoffs;
- safe delete ordering;
- batched deletes;
- DB size monitoring;
- warning/critical/emergency states.

### A20. Historical query service

- range validation;
- tier selection;
- max point enforcement;
- gap metadata;
- JSON payload contract.

### A21. Auth core

- Argon2id config;
- user schema;
- session creation;
- hashed session token storage;
- idle + absolute expiry;
- rate limiting;
- logout all.

### A22. First-run bootstrap

- setup token generation/validation;
- secret/env bootstrap;
- setup state machine;
- one-time disable behavior;
- guided diagnostic endpoints.

### A23. Frontend shell

- navigation;
- layout;
- Svelte runes stores;
- API client;
- SSE client;
- reconnect behavior;
- auth route guards.

### A24. Design tokens

- dark tokens;
- light tokens;
- status tokens;
- spacing/typography;
- focus states;
- reduced motion;
- comfortable/compact density variables.

### A25. Overview page

- server summary;
- resource health list;
- infrastructure group;
- recent events;
- loading/empty/degraded states.

### A26. Server page

- CPU composition;
- memory/swap;
- load;
- network;
- disk I/O;
- filesystems;
- collector health.

### A27. Resource pages

- lists and filters;
- detail header;
- current metrics;
- component expansion;
- history charts;
- event annotations;
- archived state.

### A28. Chart system

- shared time-series component;
- min/avg/max summaries;
- tooltip;
- gaps;
- annotations;
- responsive behavior;
- keyboard/screen-reader summary.

### A29. Settings UI

- collection settings;
- retention preset + advanced mode;
- DB budget status;
- appearance;
- auth session settings;
- effective-value source labels;
- live vs restart-required labels.

### A30. Self-monitoring page

- process CPU/RSS;
- Go runtime summary;
- DB size/WAL size;
- queue depth;
- dropped batches;
- write latency;
- collector duration;
- SSE client count.

### A31. Packaging

- minimal Dockerfile;
- Compose example;
- Coolify template;
- GHCR workflow;
- release tags/channels;
- startup environment docs.

### A32. Release qualification

- benchmark runs;
- metrics validation;
- accessibility smoke tests;
- mobile smoke tests;
- install/update/uninstall/recovery docs;
- security review;
- alpha checklist.

---

## Appendix B — Implementation Guardrails for AI Coding Agents

An agent implementing Binnacle MUST follow these guardrails:

1. Do not introduce an external database.
2. Do not introduce Redis.
3. Do not introduce Prometheus as an internal dependency.
4. Do not introduce Grafana.
5. Do not introduce a Node runtime in production.
6. Do not add Docker control operations.
7. Do not expose the raw Docker socket through Binnacle HTTP APIs.
8. Do not poll static Docker metadata every sample cycle.
9. Do not query SQLite to serve current live values.
10. Do not create unbounded channels, queues, caches, or goroutine fan-out.
11. Do not persist full application logs.
12. Do not fill chart gaps with zero.
13. Do not use container ID as long-term application identity.
14. Do not block the whole application because one collector fails.
15. Do not make Coolify API access mandatory.
16. Do not depend on undocumented Coolify database internals.
17. Do not add telemetry without explicit opt-in and project-owner approval.
18. Do not bypass config precedence or hide effective sources.
19. Do not add large UI frameworks without explicit ADR approval.
20. Do not add decorative animations that continuously consume CPU.
21. Do not claim a performance number without benchmark evidence.
22. Do not add post-alpha features into alpha.1 unless the scope decision is formally changed.
23. Do not weaken AGPL licensing or replace it with MIT/Apache without owner decision.
24. Do not change the primary product from Coolify-first Docker monitoring to generic infrastructure monitoring.
25. Prefer clear, testable, bounded implementations over flexible but abstract platforms.

---

## Appendix C — Suggested Default Alert Rules for Post-Alpha Implementation

These defaults are not part of alpha.1, but preserve the product decisions for later implementation.

### Host CPU warning

```text
Condition: cpu_busy_pct > 90
Trigger duration: 5 minutes
Recovery: cpu_busy_pct < 80 for 2 minutes
Severity: Warning
Repeat: at most every 2 hours
```

### Host memory warning

```text
Condition: memory_used_pct > 85
Trigger duration: 10 minutes
Recovery: memory_used_pct < 80 for 2 minutes
Severity: Warning
```

### Filesystem warning

```text
Condition: filesystem_used_pct > 80
Trigger duration: 5 minutes
Severity: Warning
```

### Filesystem critical

```text
Condition: filesystem_used_pct > 95
Trigger duration: 2 minutes
Severity: Critical
```

### Container restart storm

```text
Condition: > 3 restarts in 10 minutes
Severity: Warning
Deployment-aware suppression: yes
```

### OOM loop

```text
Condition: >= 2 OOM kills for same logical resource in 10 minutes
Severity: Critical
```

### Health-check failure

```text
Condition: required check fails continuously for 2 minutes
Severity: Critical
Deployment grace: 5 minutes default
Recovery: 2 consecutive successful checks or 2 minutes healthy
```

### Docker collector unavailable

```text
Condition: sustained Docker collector Down state
Severity: Critical
Transient single failures: no alert
```

These values must remain user-overridable globally and per resource.

---

## Appendix D — Suggested Coolify Template Requirements

The official Coolify template should:

- use `stable` image tag by default after stable release exists;
- use appropriate prerelease tag during alpha documentation;
- persist `/var/lib/binnacle` in a named volume;
- mount host `/proc` read-only;
- mount host `/sys` read-only;
- mount Docker socket;
- avoid privileged mode;
- avoid unnecessary capabilities;
- expose one HTTP service port;
- allow Coolify domain/HTTPS management;
- document setup token/bootstrap secret;
- document optional master encryption key;
- document resource limits;
- include health check endpoint;
- include update instructions;
- clearly warn about Docker socket privilege and hardened proxy option.

Template validation should be automated against the canonical Compose definition to avoid drift.

---

## Appendix E — Suggested Operational Status Copy

Use calm, direct language.

Examples:

```text
Healthy
All collectors are reporting current data.
```

```text
Docker metrics are temporarily unavailable.
Host monitoring is still active. Last Docker sample was 2 minutes ago.
```

```text
History recording is degraded.
Live metrics are still available, but some historical samples may be missing.
```

```text
Database volume is almost full.
Binnacle is cleaning expired data. Raw history may pause if usage reaches the emergency threshold.
```

```text
This resource is archived.
Its historical metrics are preserved, but it is no longer being monitored as an active resource.
```

```text
Deployment detected.
Expected restart alerts are temporarily suppressed during the 5-minute deployment grace period.
```

Avoid playful language, emoji, or anthropomorphic phrasing in failure states.

---

## Appendix F — Performance Review Checklist

Before merging code on a hot path, ask:

- Does this run every 2 seconds?
- Does it allocate per container?
- Can the result be cached?
- Can Docker events replace polling?
- Can serialization be shared across clients?
- Is the queue bounded?
- Does it add a database query to the live path?
- Does it scan all processes?
- Does it create one goroutine per resource?
- Does it introduce regex parsing in the sample loop?
- Does it increase metadata inspection frequency?
- Is the new metric worth its collection cost?
- Is the change benchmarked at 10, 30, 50, and 100 containers where relevant?

The default answer to unnecessary hot-path complexity is no.

---

_End of specification._
