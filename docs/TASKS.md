# TALOS v0.1.0-alpha.1 — Implementation Tasks

## How to use this backlog

This is the execution plan for the alpha.1 release defined by `docs/SPEC.md`. It is ordered by dependency and intentionally excludes post-alpha capabilities unless a task explicitly says otherwise.

Rules for every task:

- One task is one commit. Do not combine neighbouring tasks for convenience.
- Use the stated Conventional Commit subject verbatim unless a necessary implementation correction requires a more accurate scope.
- Keep the commit limited to the described behavior and its focused tests, fixtures, docs, or migration.
- Run the listed verification before committing. A later quality-gate task does not excuse a failing local check.
- **Dependency model:** `T001` through `T100` are the required merge order. A task depends on every lower-numbered task; work may be prepared in parallel only when it does not alter an earlier task's contract and it is merged in this order.
- Do not introduce workload mutation, external core services, telemetry, unbounded work queues, or a Node.js production runtime.
- Current data is served from the Metrics Engine in memory. SQLite is historical storage only.
- Record an ADR before changing a binding design decision from `docs/SPEC.md`.

### Shared acceptance standards

- New Go code is formatted, context-aware, bounded, race-safe where concurrent, and has behavioral tests for non-trivial logic.
- New browser behavior is keyboard accessible, uses semantic HTML, keeps status non-color-only, supports reduced motion, and contains localizable strings.
- All timestamps crossing an API boundary are RFC 3339 UTC; nullable measurements are `null`, never substituted with zero.
- Error responses use the documented JSON error envelope; secrets and raw application logs never appear in API responses, ordinary logs, test snapshots, or diagnostics.
- Every schema migration is forward-only, transactional where safely possible, and exercised from an empty database and every previously released alpha schema.

## Phase 0 — Repository, architecture, and developer foundation

### T001 — Establish repository policy and contributor documents

- **Status:** Complete
- **Commit:** `docs(repo): add alpha governance and contribution policies`
- **Description:** Add the public repository documents required for an AGPL, founder-led project: contribution process with DCO sign-off and Conventional Commits, code of conduct, security reporting/support policy, governance/ADR process, project notice, and a README that identifies TALOS as a temporary codename.
- **Implement:** Keep the existing AGPL-3.0 license; state the no-telemetry and permanently read-only product positions; document supported Linux/Docker scope and the public-source obligation for network-served modifications.
- **Test / verify:** Validate links and SPDX references; manually review that no document promises post-alpha features as alpha behavior.
- **Done when:** A contributor can report a vulnerability, submit a signed-off PR, understand release ownership, and identify the authoritative specification without external context.

### T002 — Create the monorepo skeleton and ignore rules

- **Status:** Complete
- **Commit:** `chore(repo): create TALOS monorepo layout`
- **Description:** Create the required `cmd`, `internal`, `web`, `migrations`, `packaging`, `scripts`, and `adr` structure with minimal package documentation; replace the generic ignore file with Go, pnpm, Vite, SQLite runtime, coverage, and local-secret exclusions.
- **Implement:** Establish only acyclic package boundaries described by the specification; add placeholder directories only where Git needs tracked files; never commit generated frontend output, database files, local profiles, or `.env` files.
- **Test / verify:** Confirm `git status --ignored` classifies expected development artifacts correctly and that no production source relies on an ignored file.
- **Done when:** The repository tree matches the architecture without inventing application logic or empty framework abstractions.

### T003 — Initialize the pinned Go module and baseline dependencies

- **Status:** Complete
- **Commit:** `build(go): initialize module and pinned toolchain`
- **Description:** Create `go.mod` for Go 1.26, add only the initial dependencies needed by the selected architecture: the CGO SQLite driver, Docker Engine client, TOML parser, and `x/crypto`.
- **Implement:** Use module paths and SPDX-compatible dependency licenses; do not add a router, ORM, migration framework, dependency-injection container, metrics suite, or logging framework without a concrete task requiring it.
- **Test / verify:** Run `go mod tidy`, `go list -m all`, and license review; build a trivial command on Linux with CGO enabled.
- **Done when:** Toolchain and dependency versions are reproducible and the dependency set stays intentionally narrow.

### T004 — Bootstrap the Svelte 5 frontend workspace

- **Status:** Complete
- **Commit:** `build(web): initialize Svelte 5 TypeScript workspace`
- **Description:** Initialize the `web` workspace with Svelte 5 runes, TypeScript, Vite, pnpm, ESLint, Prettier, Vitest, Playwright, and a committed lockfile/Corepack package-manager declaration.
- **Implement:** Configure local assets only, strict TypeScript, browser targets matching the specification, and separate development/build/test scripts. Do not include a visual component framework or external font/icon CDN.
- **Test / verify:** Run dependency installation from the lockfile, type checking, linting, unit-test bootstrap, and a production Vite build.
- **Done when:** A clean checkout can install deterministic frontend dependencies and build static assets without a Node runtime in production.

### T005 — Add the root Makefile and local development commands

- **Status:** Complete
- **Commit:** `build(dev): add reproducible Make targets`
- **Description:** Implement `make dev`, `make dev-demo`, `make dev-host`, `make test`, `make check`, and `make build` as the documented entry points.
- **Implement:** Make targets must clearly separate synthetic-demo and real-host execution, use temporary development database locations, forward failure exit codes, and avoid destructive cleanup. Stub unavailable later commands only if they fail with actionable guidance until their prerequisites are implemented.
- **Test / verify:** Invoke each target that is meaningful at this stage and assert `make help` or equivalent documents prerequisites and generated artifact locations.
- **Done when:** Contributors have one stable command surface instead of ad-hoc backend/frontend command sequences.

### T006 — Define the application lifecycle and structured logging contract

- **Status:** Complete
- **Commit:** `feat(app): add root lifecycle and structured logging`
- **Description:** Build the application composition root with a root context, structured JSON logging, component start/stop registration, SIGINT/SIGTERM handling, bounded 15-second shutdown budget, and a basic health endpoint.
- **Implement:** Shutdown order must stop HTTP acceptance, stop new collection work, await/cancel bounded in-flight work, flush persistence when available, and close storage. Log safe diagnostic fields only; never log credentials, setup tokens, session tokens, or complete Docker metadata.
- **Test / verify:** Unit-test lifecycle ordering and cancellation; integration-test SIGTERM against a test process and verify it exits within the configured budget.
- **Done when:** All long-lived services can be started from one root context and shut down predictably.

### T007 — Add architecture decision records 001–014

- **Status:** Complete
- **Commit:** `docs(adr): record initial architecture decisions`
- **Description:** Add ADRs for the binding decisions in the specification: Go, Svelte, typed SQLite, Docker/Coolify deployment, read-only operations, AGPL, SSE, single binary, Metrics Engine, identities, retention, Coolify model, telemetry, and bounded degradation.
- **Implement:** Each ADR states context, decision, consequences, and rejected alternatives; link to `SPEC.md` rather than duplicating it wholesale.
- **Test / verify:** Review ADR numbering, cross-links, and consistency with the final decision ledger.
- **Done when:** Future contributors have a concise record explaining non-obvious architectural constraints.

### T008 — Implement typed configuration defaults and validation

- **Status:** Complete
- **Commit:** `feat(config): add typed defaults and validation`
- **Description:** Define the complete typed configuration model and normative defaults for collection, persistence, retention, database budgets, API limits, sessions, Docker concurrency, paths, demo mode, and deployment-critical settings.
- **Implement:** Validate minimum collection interval, positive durations/budgets, valid retention ordering, safe queue limits, and required paths. Separate live-editable values from restart-required values in the type model.
- **Test / verify:** Table-test defaults, invalid combinations, minimum intervals, and error messages; assert all specification defaults are represented exactly.
- **Done when:** No component owns hidden configuration defaults or accepts unsafe values.

### T009 — Implement TOML, environment, and effective-source configuration loading

- **Status:** Complete
- **Commit:** `feat(config): load TOML and environment overrides`
- **Description:** Load TOML from the documented discovery order or `TALOS_CONFIG_FILE`, map environment variables, and report an effective value plus source for every setting.
- **Implement:** Apply precedence `defaults < file < environment < persisted eligible override`; prevent persisted overrides from changing paths, listen address, master key, Docker socket, or host proc/sys mounts. Treat missing optional files as normal and malformed configured files as startup failures with safe diagnostics.
- **Test / verify:** Test discovery order, override precedence, source labels, unknown-key policy, and redaction of secret values.
- **Done when:** Settings consumers receive one validated effective configuration and can explain where it came from.

### T010 — Build the SQLite connection and migration framework

- **Status:** Complete
- **Commit:** `feat(storage): add SQLite bootstrap and migrations`
- **Description:** Add a CGO SQLite connection manager, WAL-oriented pragmas, busy timeout, foreign keys, integrity preflight, migration ledger, embedded SQL migrations, schema-version reporting, and migration failure handling.
- **Implement:** Use one controlled connection policy suitable for a single writer and concurrent readers. Before migration, check integrity and disk availability; do not crash-loop or recreate a failed/corrupt database.
- **Test / verify:** Test fresh migration, idempotent reopen, failed migration reporting, integrity failure behavior, and pragma application using temporary databases.
- **Done when:** Startup can safely prepare a new database and preserve a failed database for recovery.

### T011 — Define core domain types and nullable metric semantics

- **Status:** Complete
- **Commit:** `feat(metrics): define observations snapshots and events`
- **Description:** Introduce shared typed domain models for host/container observations, metadata, logical resources, current snapshots, collector health, events, persistence batches, time ranges, units, and explicit missing values.
- **Implement:** Keep Docker IDs ephemeral and logical resource IDs stable; use UTC timestamps plus monotonic elapsed durations for rate calculations; include boot identity, freshness, and monotonic snapshot sequence fields.
- **Test / verify:** Compile-time/package tests for JSON null encoding, unit naming, status enums, and invalid state rejection.
- **Done when:** Collectors, storage, API, and UI adapters share one precise data vocabulary without generic metric-name/value blobs.

### T012 — Add HTTP foundation, error handling, and request limits

- **Status:** Complete
- **Commit:** `feat(api): add HTTP foundation and safe error responses`
- **Description:** Implement the `net/http` server foundation, route registration, JSON response helpers, RFC 3339 encoding, error envelope, request IDs, body-size limits, and recovery middleware.
- **Implement:** Reserve `/api/v1/` and static frontend routing; add no unrestricted Docker proxying. Centralize query parsing and prohibit cacheable authenticated responses by default.
- **Test / verify:** Test malformed JSON, oversized bodies, panics, method handling, error envelope shape, and timestamp/nullable encoding.
- **Done when:** Later APIs inherit consistent safe behavior rather than reproducing parsing and error logic.

### T013 — Embed production web assets into the Go binary

- **Status:** Complete
- **Commit:** `build(web): embed compiled frontend assets`
- **Description:** Connect Vite production output to Go `embed`, serve immutable hashed assets with suitable cache headers, and fall back to the SPA entrypoint only for recognized UI routes.
- **Implement:** Ensure API routes are never swallowed by SPA fallback; serve no external scripts, fonts, or icons; make missing/invalid embedded assets a build failure.
- **Test / verify:** Build frontend then Go binary, smoke-test asset serving, deep-link routing, MIME types, and API 404 behavior.
- **Done when:** A single binary can serve the complete frontend offline.

### T014 — Establish CI quality-gate skeleton

- **Status:** Complete
- **Commit:** `ci: add baseline verification workflows`
- **Description:** Add GitHub Actions for Go formatting/vet/unit tests, frontend format/lint/type/unit checks, dependency lockfile validation, container build smoke test, and documentation link checks.
- **Implement:** Pin action versions by immutable reference where practical, use least-privilege permissions, cache only safe build dependencies, and make the workflow extensible for later Playwright, scans, SBOM, and release jobs.
- **Test / verify:** Validate workflow syntax and run equivalent local checks; confirm no workflow publishes artifacts or contacts external services on pull requests.
- **Done when:** A PR has a reliable minimum quality signal from the first implementation commit.

## Phase 1 — Demo mode, live transport, and frontend shell

### T015 — Add deterministic demo clock and scenario generator

- **Commit:** `feat(demo): add seeded synthetic monitoring scenarios`
- **Description:** Implement `talos --demo` and seedable deterministic host/resource/event scenarios that require neither Docker nor host mounts.
- **Implement:** Model normal load, deployment overlap, restart, OOM, collector degradation, archived resource, and recovery states using a controllable clock. Ensure generated data contains no real host identifiers or secrets.
- **Test / verify:** Assert identical seed/time inputs produce identical snapshots/events and different seeds remain structurally valid.
- **Done when:** Frontend development, visual tests, and documentation can run independently from a monitored server.

### T016 — Implement the Metrics Engine current-state event loop

- **Commit:** `feat(metrics): add in-memory current-state engine`
- **Description:** Create the central Metrics Engine that ingests typed collector messages, tracks current host/container/resource state, freshness, metadata, events, boot identity, and sequence numbers.
- **Implement:** Use bounded channels or a carefully synchronized store; publish immutable snapshots; do not write SQLite or serialize HTTP payloads in collector paths; preserve unknown/missing values rather than manufacturing zeroes.
- **Test / verify:** Test concurrent ingestion, monotonic sequence generation, stale freshness behavior, immutable snapshots, and recovery after partial collector failure.
- **Done when:** Every live consumer has one authoritative in-memory state source.

### T017 — Add snapshot subscriptions and bounded fan-out

- **Commit:** `feat(metrics): add bounded live snapshot subscriptions`
- **Description:** Add subscription lifecycle management to the Metrics Engine for snapshot and discrete-event consumers.
- **Implement:** Give each subscriber a bounded output buffer; replace stale queued snapshots with the newest one; retain discrete events where feasible; disconnect or cancel persistently slow subscribers without delaying collection.
- **Test / verify:** Simulate slow and disconnected subscribers, verify no producer blocks, and race-test subscribe/unsubscribe during publish.
- **Done when:** Browser load cannot create unbounded memory, goroutines, or collector latency.

### T018 — Implement authenticated SSE transport semantics

- **Commit:** `feat(api): stream live snapshots over SSE`
- **Description:** Implement `/api/v1/live` with `snapshot`, `event`, `collector_state`, and heartbeat frames, reconnect-safe IDs, and a 15–30 second keepalive.
- **Implement:** Serialize a compact snapshot once per publish for shared fan-out; omit static metadata blobs; apply the authentication hook before stream creation; set proxy-safe SSE headers and cancel work on client disconnect.
- **Test / verify:** HTTP-test event framing, heartbeat, reconnect ID behavior, slow-client handling, cancellation, and shared serialization instrumentation.
- **Done when:** A browser can receive fresh current state every two seconds without database reads or per-client collector work.

### T019 — Build the typed frontend API and SSE client

- **Commit:** `feat(web): add typed API and live-stream client`
- **Description:** Add a TypeScript API client, typed API models, an SSE client with reconnect/backoff, and runes-based current-state store.
- **Implement:** Handle authentication failure, network loss, stale data, event de-duplication, and reconnect status visibly; never use a polling fallback that duplicates live work.
- **Test / verify:** Unit-test model decoding and reconnect state transitions; browser-test reconnect after a dropped stream.
- **Done when:** All pages can consume one shared live state source with explicit connection health.

### T020 — Implement the application shell and route guards

- **Commit:** `feat(web): add authenticated application shell`
- **Description:** Build the primary navigation, responsive shell, page routing, loading/error boundaries, login/setup route guards, and unavailable-state handling.
- **Implement:** Use semantic landmarks and skip navigation; include Overview, Resources, Server, Events, Checks placeholder, and Settings; do not expose protected content before auth state resolves.
- **Test / verify:** Playwright-test unauthenticated redirects, active navigation, keyboard skip link, deep-link refresh, and narrow viewport shell behavior.
- **Done when:** The application has a stable, accessible navigation frame ready for real pages.

### T021 — Establish design tokens, themes, density, and motion policy

- **Commit:** `feat(web): add TALOS design tokens and preferences`
- **Description:** Create project-owned color, typography, spacing, elevation, status, focus, and chart tokens; implement System/Dark/Light theme selection and Comfortable/Compact density preference.
- **Implement:** Persist display preferences locally or in eligible settings later; honor `prefers-reduced-motion`; use text/icon/color for all status states; keep fonts local/system-only.
- **Test / verify:** Unit-test preference resolution; browser-test theme persistence, contrast-sensitive status rendering, compact layout, and reduced-motion class behavior.
- **Done when:** New UI uses a consistent “telemetry console with a terminal soul” system rather than generic component defaults.

### T022 — Create reusable accessible UI primitives

- **Commit:** `feat(web): add accessible core UI primitives`
- **Description:** Build small project-owned primitives for buttons, form fields, badges, alerts, disclosure panels, dialogs, menus, tabs, empty states, loading states, and technical-detail drawers.
- **Implement:** Use native semantics first; implement focus trapping/return only where dialogs require it; do not add a broad component framework. All strings must use translation keys from the start.
- **Test / verify:** Unit-test keyboard activation and ARIA behavior; run axe smoke checks against representative primitive combinations.
- **Done when:** Feature pages can be built consistently without copying unsafe interaction logic.

### T023 — Add localization-ready message and formatting infrastructure

- **Commit:** `feat(web): add localization-ready formatting layer`
- **Description:** Add English message resources, stable message keys, locale-aware number/date/duration/byte/rate formatters, and a translation-loading boundary.
- **Implement:** Prohibit concatenated translated fragments and hard-coded user-facing English inside components; keep right-to-left support out of alpha while avoiding layout assumptions that prevent it.
- **Test / verify:** Test formatting across representative locale inputs and missing-key development failure behavior.
- **Done when:** Alpha ships English while future translations do not require component rewrites.

### T024 — Build the shared uPlot chart foundation

- **Commit:** `feat(web): add incremental time-series chart foundation`
- **Description:** Wrap uPlot in a Svelte component with stable lifecycle, resize handling, theme tokens, line/area/sparkline variants, tooltip/focus inspection, min/avg/max summary, and an accessible text summary.
- **Implement:** Update series incrementally; never destroy/recreate a chart every live tick; render missing samples as gaps and support event/deployment markers; cap input at backend-provided point limits.
- **Test / verify:** Unit-test data-to-series conversion and gap handling; Playwright-test resize, keyboard/focus summary, reduced motion, and incremental update instrumentation.
- **Done when:** Every historical/live chart has a lightweight, accessible, visually consistent base.

### T025 — Deliver the demo overview vertical slice

- **Commit:** `feat(web): add demo overview live dashboard`
- **Description:** Implement a demo-backed overview showing host health summary, resource status/CPU/memory, infrastructure grouping, recent events, collector state, and clear loading/degraded/empty states.
- **Implement:** Make the first viewport answer server health, affected resources, and recent change within five seconds; avoid cockpit-density and decorative gauges.
- **Test / verify:** Playwright-test deterministic demo states, status transitions, mobile viewport, and no chart recreation across several SSE updates.
- **Done when:** The product can be demonstrated convincingly before real collectors exist.

## Phase 2 — Host collection and server telemetry

### T026 — Parse `/proc/stat` and compute host CPU deltas

- **Commit:** `feat(host): collect host CPU utilization`
- **Description:** Implement fixture-driven `/proc/stat` parsing and delta calculations for total/per-core busy, user, system, iowait, steal, and online CPU count.
- **Implement:** Exclude guest/guest_nice from duplicated total accounting; treat zero/negative deltas and counter reset boundaries as missing samples; resolve paths through configured host proc mount.
- **Test / verify:** Table-test kernel field variants, first sample, per-core changes, reset/zero deltas, and formulas against known fixtures.
- **Done when:** CPU values have documented Linux semantics and never convert invalid counters into false zero load.

### T027 — Collect host memory, swap, load, uptime, and boot identity

- **Commit:** `feat(host): collect memory load and boot state`
- **Description:** Add `/proc/meminfo`, `/proc/loadavg`, `/proc/uptime`, and boot-identity collection with normalized memory/swap/load observations.
- **Implement:** Use `MemTotal - MemAvailable` as primary used memory; preserve cache/buffer details separately; detect reboot boundaries, emit an event, and prevent counter continuity across boots.
- **Test / verify:** Fixture-test absent optional fields, zero totals, memory formulas, malformed input, uptime parsing, and boot transition events.
- **Done when:** Host memory and uptime state remains useful across normal Linux variation and reboots.

### T028 — Collect and classify host network interfaces

- **Commit:** `feat(host): collect network interface rates`
- **Description:** Parse `/proc/net/dev`, compute byte/packet/error/drop rates, and classify interfaces for aggregate versus detailed display.
- **Implement:** Exclude loopback by default and avoid obvious Docker bridge/veth double counting in host aggregate while retaining interface detail; counter decreases create reset boundaries.
- **Test / verify:** Use fixtures for physical, loopback, bridge, veth, reset, and malformed interfaces; assert aggregate totals and null gap semantics.
- **Done when:** Network panels distinguish meaningful host traffic from container-network implementation detail.

### T029 — Collect and classify host disk I/O

- **Commit:** `feat(host): collect disk I/O rates and devices`
- **Description:** Parse `/proc/diskstats`, resolve block metadata from host sysfs, calculate read/write bytes and operations per second, and provide relevant device aggregation.
- **Implement:** Centralize Linux sector conversion; exclude loop/ram/synthetic devices by default; avoid double-counting whole disks and partitions; preserve per-device detail and reset handling.
- **Test / verify:** Fixture-test device classification, partition relationships, sector conversion, IOPS, counter resets, and missing sysfs metadata.
- **Done when:** Disk I/O values are correct, explainable, and not inflated by device topology.

### T030 — Collect filesystems and inode capacity

- **Commit:** `feat(host): collect filesystem and inode capacity`
- **Description:** Discover host-visible mount points and collect total/used/available bytes and inode usage via statfs-equivalent calls.
- **Implement:** Filter pseudo/overlay-internal filesystems from overview; always retain root and TALOS data-volume filesystems; expose source, mount, and filesystem type for advanced view.
- **Test / verify:** Test mount filtering, stat failures, inode-unavailable filesystems, root/data-volume inclusion, and stable mount keys.
- **Done when:** Disk-capacity monitoring focuses on actionable filesystems without hiding critical storage.

### T031 — Implement resilient host collector scheduling and health

- **Commit:** `feat(host): schedule host collection with health states`
- **Description:** Compose host subcollectors into the configured interval collector with independent optional-field failure handling, durations, freshness, and stateful healthy/degraded/down transitions.
- **Implement:** A missing optional Linux source must not erase available host metrics; respect root-context cancellation and minimum interval; publish collector-state events only after transient-failure policy is exceeded.
- **Test / verify:** Fake-clock test scheduling, cancellation, partial failures, sustained failure/recovery, and duration/self-metric recording.
- **Done when:** Host collection degrades honestly without taking down live monitoring.

### T032 — Persist host identity and boot sessions

- **Commit:** `feat(storage): persist host and boot entities`
- **Description:** Add migrations and repository operations for `hosts` and `boot_sessions`, including hashed machine identity where available and boot-session lifecycle updates.
- **Implement:** Store only safe host identity metadata; create/close boot sessions deterministically; keep writer ownership rules intact.
- **Test / verify:** Migration tests plus repository tests for first observation, repeat observation, reboot, and host metadata update.
- **Done when:** Historical host samples can be tied to a stable host and correct boot boundary.

### T033 — Expose current server summary and collector health APIs

- **Commit:** `feat(api): add server and collector-health endpoints`
- **Description:** Implement authenticated `GET /api/v1/server` and `GET /api/v1/collector-health` backed only by the Metrics Engine.
- **Implement:** Return documented CPU/memory/load/freshness contracts, explicit nulls, independent collector states/reasons, and no storage query on the hot path.
- **Test / verify:** Contract-test normal, stale, unavailable, and unauthenticated responses; instrument to prove no SQLite call occurs.
- **Done when:** The frontend can display trustworthy host state before historical storage is queried.

### T034 — Build the server page current-metric panels

- **Commit:** `feat(web): add server telemetry page`
- **Description:** Implement server page panels for CPU composition/per-core detail, memory/swap, load, disk I/O, network, filesystems/inodes, uptime/boot state, and collector health.
- **Implement:** Label iowait and steal clearly; distinguish unavailable from zero; use progressive disclosure for per-device/interface details and responsive layouts.
- **Test / verify:** Component tests for null/stale states; Playwright keyboard, mobile, dark/light, and compact-density smoke paths.
- **Done when:** A user can diagnose basic host pressure without terminal tools.

## Phase 3 — Docker discovery, statistics, and events

### T035 — Create the narrow read-only Docker client boundary

- **Commit:** `feat(docker): add read-only Engine API client`
- **Description:** Wrap the Docker Engine client in interfaces exposing only list, inspect, stats, events, version/system diagnostics, and later bounded logs—not mutation methods.
- **Implement:** Centralize socket path, context deadlines, Docker API version handling, error normalization, and concurrency cap; prevent the rest of the codebase from accessing the raw Docker client.
- **Test / verify:** Compile-time interface tests, fake-client tests, timeout/error mapping tests, and static search proving mutation methods are absent from production code.
- **Done when:** Docker socket privilege is constrained by application design as far as code can enforce it.

### T036 — Implement Docker startup discovery and metadata model

- **Commit:** `feat(docker): discover containers and cache metadata`
- **Description:** Discover containers at startup and build sanitized metadata records containing IDs, names, labels, images, timestamps, state, health, Compose labels, networks, and safe mount metadata.
- **Implement:** Do not retain environment variables or secret-bearing inspect fields; populate cache misses only through controlled inspection; represent stopped/destroyed timestamps explicitly.
- **Test / verify:** Fixture-test inspect/list decoding, metadata sanitization, absent labels, and startup discovery errors.
- **Done when:** Static metadata is available to resolution logic without being re-inspected each sampling cycle.

### T037 — Add Docker event subscription and cache reconciliation

- **Commit:** `feat(docker): maintain event-driven metadata cache`
- **Description:** Subscribe to Docker lifecycle events, update/invalidate metadata cache, reconnect with bounded backoff, and reconcile at the configured low-frequency interval.
- **Implement:** Handle create/start/stop/die/destroy/rename/health/OOM events; deduplicate replayed events safely; reconciliation corrects missed events without clearing current state prematurely.
- **Test / verify:** Replay event fixtures through fake clock/client, test reconnect, duplicate events, reconciliation after loss, and cancellation.
- **Done when:** Metadata stays accurate under deployment churn without a two-second inspect loop.

### T038 — Normalize Docker CPU metrics

- **Commit:** `feat(docker): normalize container CPU metrics`
- **Description:** Convert Docker stats deltas into host-normalized CPU percent, Docker-style percent, and core equivalents with explicit online-CPU semantics.
- **Implement:** Use host-normalized percentage as default; preserve legitimate multi-core usage in core equivalents; null invalid/reset intervals and clamp only display values later.
- **Test / verify:** Fixture-test cgroup/Docker variants, CPU count changes, first sample, zero system delta, counter reset, and formula equivalence.
- **Done when:** Container CPU charts use unambiguous, validated semantics.

### T039 — Normalize Docker memory and PID metrics

- **Commit:** `feat(docker): normalize container memory and PIDs`
- **Description:** Collect raw memory usage, cgroup-v2 working set where available, finite limit, host-relative fallback denominator, memory percent, and PID count.
- **Implement:** Calculate working set as `max(0, current - inactive_file)` where supported; label unlimited-limit comparisons accurately; do not claim precision unavailable from an Engine payload.
- **Test / verify:** Test cgroup-v1/v2-like fixtures, absent/infinite limits, inactive-file underflow, and PID absence.
- **Done when:** Memory is operationally meaningful rather than a misleading cache-inclusive number.

### T040 — Normalize Docker network and block-I/O rates

- **Commit:** `feat(docker): normalize container network and block I/O`
- **Description:** Sum per-container interface counters and block-I/O counters, compute RX/TX and read/write rates, and preserve reset boundaries.
- **Implement:** Treat a replacement as a new instance series; do not merge distinct instance counters before resource aggregation; keep fields null when Docker omits a metric.
- **Test / verify:** Fixture-test multiple interfaces/devices, resets, partial stats, and rates using controlled elapsed time.
- **Done when:** Container resource traffic and I/O are safe to aggregate across components.

### T041 — Implement bounded concurrent Docker stats collection

- **Commit:** `feat(docker): collect container stats with concurrency limits`
- **Description:** Schedule running-container stats reads every configured interval with a maximum of four concurrent Engine requests by default, cancellation, durations, and per-container partial failures.
- **Implement:** Reuse metadata cache; do not create an unbounded goroutine per container; retain recent known lifecycle state when a single stats request fails while exposing freshness correctly.
- **Test / verify:** Fake-client concurrency test, cancellation test, partial-failure test, and benchmark at 10/30/50/100 synthetic containers.
- **Done when:** Docker metric collection remains bounded and measurable as container count grows.

### T042 — Normalize lifecycle, health, and OOM events

- **Commit:** `feat(events): normalize Docker lifecycle and OOM events`
- **Description:** Convert Docker events and inspected state into stable event types, severity, safe details, correlation keys, health transitions, restart/replacement hints, and OOM deduplication.
- **Implement:** Deduplicate `oom`, `die` with `OOMKilled`, and cgroup evidence into one user-visible OOM; retain raw diagnostic references only when non-sensitive.
- **Test / verify:** Fixture-test every required event mapping, duplicate OOM signals, malformed event data, and severity/correlation outcomes.
- **Done when:** Events become concise operational facts rather than Docker-specific payload dumps.

### T043 — Add Docker collector health and outage semantics

- **Commit:** `feat(docker): isolate Docker collector failures`
- **Description:** Implement Docker collector healthy/degraded/down state transitions, freshness aging, recovery events, and continued host monitoring during Engine outage.
- **Implement:** Avoid user-visible degradation for a single transient timeout; clear stale resource current values to Unknown once freshness expires rather than presenting old metrics as live.
- **Test / verify:** Integration-test outage/recovery sequence, collector-state API payload, resource stale transitions, and host collector continuity.
- **Done when:** Docker failure matches the documented graceful-degradation scenario.

## Phase 4 — Logical resources and Coolify-aware grouping

### T044 — Add resource and container-instance schema

- **Commit:** `feat(storage): add resource identity schema`
- **Description:** Add typed migrations for `resources` and `container_instances`, required indexes, source kinds, archive fields, category override fields, and sanitized metadata JSON.
- **Implement:** Enforce unique `(host_id, stable_key)` identity; use container ID only as the instance primary key; preserve historical instances after replacement.
- **Test / verify:** Migration and repository tests for uniqueness, references, archive timestamps, and safe metadata serialization.
- **Done when:** Storage can represent stable logical resources across ephemeral containers.

### T045 — Resolve Compose logical resource identities

- **Commit:** `feat(resources): resolve Compose resource identities`
- **Description:** Implement standard Compose-label resolution using project, service, and container-number metadata with deterministic stable keys and names.
- **Implement:** Support missing/partial labels without panicking; never concatenate unchecked labels into unsafe API identifiers; preserve enough source context for UI grouping.
- **Test / verify:** Fixture-test normal Compose deployments, replicas, label absence, renamed containers, and deterministic key generation.
- **Done when:** Plain Docker Compose hosts receive useful service-oriented grouping.

### T046 — Add isolated Coolify-aware resolver rules

- **Commit:** `feat(coolify): resolve Coolify resource metadata`
- **Description:** Add fixture-driven Coolify label mapping in the resolver layer, including project/environment/name signals and infrastructure classification, without accessing Coolify internals.
- **Implement:** Keep undocumented label mappings isolated and versioned by fixtures; fall back cleanly to Compose/derived identity when labels are absent or change.
- **Test / verify:** Test supported Coolify metadata samples, missing/changed labels, fallback order, and no dependency on Coolify database files.
- **Done when:** TALOS is Coolify-first while remaining operational when Coolify metadata is incomplete.

### T047 — Implement stable identity fallback and manual category overrides

- **Commit:** `feat(resources): add identity fallback and category overrides`
- **Description:** Implement precedence of Coolify UUID, Compose project/service, stable derived metadata, and persisted manual mapping; persist user category override across redeployments.
- **Implement:** Derived keys must be deterministic, collision-resistant within a host, and never based solely on container ID; define override validation against alpha categories.
- **Test / verify:** Test each precedence branch, collisions, replacement persistence, invalid overrides, and user override superseding inferred category only.
- **Done when:** Logical history survives replacement and users can correct classification safely.

### T048 — Build resource membership and multi-container aggregation

- **Commit:** `feat(resources): aggregate logical resource metrics`
- **Description:** Map current instances to resources and aggregate CPU, working-set memory, raw memory, limits, network, block I/O, PIDs, active-instance count, and component detail once per sample cycle.
- **Implement:** Sum actual overlapping old/new instance usage; preserve component-level metrics and missing-value semantics; do not aggregate unavailable values as zero.
- **Test / verify:** Test one/many components, overlap during redeploy, partial metric absence, membership change, and aggregation reuse by snapshot/persistence consumers.
- **Done when:** Resource summaries remain continuous and honest during multi-container rollouts.

### T049 — Implement deterministic resource status rollups

- **Commit:** `feat(resources): add resource status rollups`
- **Description:** Derive alpha resource states from container/lifecycle/collector signals using documented precedence `Down > Degraded > Unknown > Paused > Healthy`.
- **Implement:** Keep health-check state out of alpha; attach the most-specific failing component/event while reflecting parent status; make intentional stop/paused behavior explicit.
- **Test / verify:** Table-test every precedence conflict, stale Docker data, mixed components, and transition events.
- **Done when:** Resource status is deterministic and never overclaims health.

### T050 — Detect replacements, deployments, and rollout overlap

- **Commit:** `feat(events): correlate deployments and replacements`
- **Description:** Correlate image changes, old-stop/new-create patterns, Compose reconciliation, lifecycle timing, and optional future enrichment hooks into Confirmed/Likely/Container replacement annotations.
- **Implement:** Model old/new active overlap, start/stop times, peak aggregate use, and correlation windows; do not enable alerts or grace periods in alpha.
- **Test / verify:** Fixture-test simple replacement, likely deployment, unrelated containers, overlap, duplicate signals, and confidence classification.
- **Done when:** Charts and event timelines can explain deployment-related changes without pretending certainty.

### T051 — Archive disappeared logical resources

- **Commit:** `feat(resources): archive removed logical resources`
- **Description:** After reconciliation confirms a resource has disappeared, mark it Archived, preserve history, disable active monitoring membership, and hide it from default current lists.
- **Implement:** Avoid archiving during transient Docker outages or rollout windows; retain last-seen timestamp and allow later manual purge through a separate task.
- **Test / verify:** Test disappearance confirmation, recovery before archive, Docker outage, archived list filtering, and replacement under same stable key.
- **Done when:** Removed applications remain historically inspectable without cluttering active monitoring.

### T052 — Expose resource list and detail current-state APIs

- **Commit:** `feat(api): add current resource endpoints`
- **Description:** Implement authenticated `GET /api/v1/resources` and `GET /api/v1/resources/{id}` with stable metadata, current aggregate metrics, component detail, status, archive state, and freshness.
- **Implement:** Support bounded category/archive filtering and stable ordering; return 404 for unknown IDs without revealing unrelated data; read only Metrics Engine state plus controlled metadata repository data.
- **Test / verify:** Contract-test active/archived/filter states, null metrics, stale state, component ordering, and authorization.
- **Done when:** Resource UI has a safe complete current-state contract.

### T053 — Build resource navigation and list views

- **Commit:** `feat(web): add resource list navigation`
- **Description:** Implement Projects, Applications, Services, Infrastructure, Unmanaged, and Archived resource views with status, current metrics, grouping, empty states, and filters.
- **Implement:** Containers remain a component detail, not top-level navigation; ensure archived resources are excluded from default overview/list but reachable intentionally.
- **Test / verify:** Component and Playwright tests for filters, status text/icon/color, keyboard navigation, compact density, and mobile list rendering.
- **Done when:** Users navigate by logical service rather than raw container IDs.

### T054 — Build resource detail current-state and component views

- **Commit:** `feat(web): add resource detail current panels`
- **Description:** Implement resource header/context, current CPU/memory/network/block-I/O summaries, status explanation, expandable component list, deployment/replacement summary, events preview, and metadata drawer.
- **Implement:** Clearly label host-normalized CPU; keep raw IDs/paths in monospace technical disclosure; show Unknown/stale/Archived states explicitly.
- **Test / verify:** Playwright-test multi-component expansion, archived resource rendering, stale data, deployment overlap, keyboard drawer handling, and narrow viewport behavior.
- **Done when:** Users can move from an unhealthy resource to the responsible component without visual clutter.

## Phase 5 — Persistence, events, and degradation

### T055 — Add raw sample, event, collector-state, and settings schema

- **Commit:** `feat(storage): add alpha telemetry tables`
- **Description:** Add typed migrations for raw host/resource/optional instance samples, filesystem/interface samples, events, collector-state events, settings, encrypted secrets, users, and sessions with required indexes.
- **Implement:** Follow the normative schemas; use nullable metric columns and integer Unix milliseconds; do not use EAV storage or persist application logs.
- **Test / verify:** Migration test inspects tables/indexes/constraints and verifies representative nullable inserts and foreign-key enforcement.
- **Done when:** Alpha storage is compact, queryable, and structurally ready for auth/settings without schema shortcuts.

### T056 — Build immutable persistence batches and scheduler

- **Commit:** `feat(metrics): create scheduled persistence batches`
- **Description:** Have the Metrics Engine create immutable 10-second batches containing host/resource/optional instance records, slower filesystem/interface samples, pending normalized events, and collector state changes.
- **Implement:** Use one normalized/aggregated snapshot per cycle; avoid persistence when no meaningful sample exists; preserve explicit gaps and boot IDs.
- **Test / verify:** Fake-clock test batch cadence, immutability, filesystem one-minute cadence, event inclusion once, and missing data representation.
- **Done when:** The writer receives ordered storage-ready facts without re-reading collectors or recalculating resources.

### T057 — Add bounded persistence queue and overflow behavior

- **Commit:** `feat(metrics): bound persistence backlog and overflow`
- **Description:** Add the 60-batch persistence queue with oldest-batch drop policy, queue depth, dropped-batch counter, degraded state, and recovery notification.
- **Implement:** Producers never block indefinitely on writer slowness; queue overflow must retain the newest current history opportunity and create a user-visible internal event/state.
- **Test / verify:** Test full queue ordering, concurrent producers, overflow counter/event, recovery after drain, and no unbounded allocation.
- **Done when:** Storage trouble cannot consume process memory or stop live monitoring.

### T058 — Implement single-owner SQLite batch writer

- **Commit:** `feat(storage): write telemetry batches transactionally`
- **Description:** Implement the one-writer goroutine/repository that writes each batch with prepared typed statements in bounded transactions.
- **Implement:** Serialize settings/auth writes through controlled ownership; record write latency; never let collectors or HTTP handlers issue arbitrary concurrent writes; retain transaction order.
- **Test / verify:** Integration-test host/resource/event inserts, rollback on injected failure, concurrent settings request serialization, and write-latency metrics.
- **Done when:** Historical writes are efficient and ownership is enforceable.

### T059 — Add writer retry, disk-full, and corruption degradation handling

- **Commit:** `feat(storage): degrade safely on persistence failures`
- **Description:** Add exponential backoff with jitter for transient writer failures, disk-full/I/O state reporting, and corruption behavior that stops unsafe writes without deleting data.
- **Implement:** Keep collectors/SSE alive, retain bounded queued batches, expose `History persistence degraded`, and avoid crash loops/recreate-on-corruption behavior.
- **Test / verify:** Inject busy, full, I/O, and corruption-like errors; assert retry limits/backoff, live snapshot continuity, queue behavior, and recovery event.
- **Done when:** The SQLite disk-full acceptance scenario passes end to end.

### T060 — Persist normalized events and collector transitions

- **Commit:** `feat(storage): persist events and collector health history`
- **Description:** Write normalized events and state transitions with correlation keys, safe details JSON, severity, source, and resource/instance references.
- **Implement:** Ensure event insertion is idempotent under retry/replay where IDs are reused; retain recent in-memory event ring for live UI while SQLite serves history.
- **Test / verify:** Test duplicate retries, ordering, null references, safe detail sanitization, and recent-event ring eviction.
- **Done when:** Events are reliable across both live and historical views.

### T061 — Implement self-observation instrumentation

- **Commit:** `feat(app): collect TALOS self-observation metrics`
- **Description:** Measure TALOS CPU/RSS, Go heap/goroutines, database/WAL size, write/rollup/retention durations, samples/sec, collector durations, dropped batches, queue depth, SSE clients, and Docker request duration/errors.
- **Implement:** Keep collection lightweight and internal; avoid recursively persisting high-rate self-metrics into unbounded storage; make values available to diagnostics and current UI.
- **Test / verify:** Unit-test counter/gauge updates and filesystem-size failures; integration-test active SSE and queue metrics.
- **Done when:** TALOS can prove and expose its own operating cost from alpha onward.

### T062 — Add events history API with validated filtering

- **Commit:** `feat(api): add events history endpoint`
- **Description:** Implement authenticated `GET /api/v1/events` over SQLite with bounded time range, resource/type/severity/source filters, cursor or limit pagination, and stable chronological ordering.
- **Implement:** Validate all filters, cap response size, use indexes, return safe expanded details only on request, and avoid leaking secrets from stored payloads.
- **Test / verify:** Contract-test invalid ranges/filters, pagination, archive/resource filtering, authorization, and query-plan/index use on fixture data.
- **Done when:** Event history supports operational investigation without an unbounded query surface.

### T063 — Build the events page

- **Commit:** `feat(web): add searchable events timeline`
- **Description:** Implement chronological event list with time, resource, type, severity, and source filters; concise summary; expandable technical details; and loading/empty/error states.
- **Implement:** Preserve event text context through filtering and link resource events to resource detail; distinguish live arrival from persisted history without duplicates.
- **Test / verify:** Playwright-test filters, expand/collapse keyboard behavior, pagination/load more, unknown resource, and mobile layout.
- **Done when:** A user can answer what changed without reading Docker logs.

## Phase 6 — History, rollups, retention, and data lifecycle

### T064 — Add typed one-minute rollup tables and worker

- **Commit:** `feat(rollup): aggregate one-minute telemetry rollups`
- **Description:** Add typed host/resource one-minute rollup schema and worker that processes only closed raw buckets into min/max/avg/sample-count rows.
- **Implement:** Use deterministic UTC bucket starts and idempotent upsert behavior; never count null/missing samples as zero; record duration and failure health.
- **Test / verify:** Test complete/partial/missing buckets, repeat execution idempotency, boundary timestamps, and raw-to-rollup numerical correctness.
- **Done when:** Raw 10-second data has a safe first downsampling tier.

### T065 — Add fifteen-minute and hourly rollup tiers

- **Commit:** `feat(rollup): add long-range telemetry rollups`
- **Description:** Extend rollups to typed 15-minute and one-hour host/resource tables sourced from completed lower tiers or raw data where required.
- **Implement:** Preserve min/max/avg/sample-count semantics across tiers; do not prematurely delete upstream data; make schema/query fields consistent across resolutions.
- **Test / verify:** Test cross-tier calculations, partial upstream buckets, idempotency, and recovery after interrupted worker runs.
- **Done when:** Historical queries can remain bounded across long retention periods.

### T066 — Implement retention presets and advanced retention validation

- **Commit:** `feat(retention): add retention presets and overrides`
- **Description:** Implement Minimal, Balanced, and Long-term presets plus validated advanced per-tier duration overrides.
- **Implement:** Enforce downstream retention not shorter than required rollup production horizon; expose effective setting source and mark UI-editable values; do not silently reset a custom plan when preset changes.
- **Test / verify:** Test all normative presets, invalid orderings, custom override persistence, and effective configuration display models.
- **Done when:** Users can choose storage history intentionally without breaking rollup correctness.

### T067 — Implement safe bounded retention deletion

- **Commit:** `feat(retention): delete expired telemetry safely`
- **Description:** Add hourly retention worker that verifies eligible downstream rollups, deletes expired rows in bounded batches, yields between transactions, and reports work/health.
- **Implement:** Delete only after destination tier is confirmed; keep settings/auth/recent events protected; make cancellation safe and resumable.
- **Test / verify:** Test deletion ordering, batch boundaries, interrupted deletion, no-rollup refusal, and retention cutoff correctness.
- **Done when:** Database growth is controlled without silently losing still-required data.

### T068 — Add database budget monitoring and emergency policy

- **Commit:** `feat(storage): enforce database budget safeguards`
- **Description:** Monitor database/WAL size against target, warning, critical, and emergency ratios; trigger aggressive expired cleanup at critical and pause highest-resolution raw persistence only at emergency.
- **Implement:** Never silently delete in-retention data to satisfy soft budget; preserve settings/auth/events/aggregates before sacrificing oldest queued raw samples; surface current state/reason.
- **Test / verify:** Simulate all threshold transitions, cleanup request, emergency pause/resume, and priority ordering.
- **Done when:** Storage pressure is visible, controlled, and aligned with documented data priorities.

### T069 — Implement historical metrics query service

- **Commit:** `feat(storage): query historical metrics by resolution`
- **Description:** Implement validated historical host/resource metrics queries with automatic raw/1m/15m/1h resolution selection and approximately 1000-point cap.
- **Implement:** Apply documented range mapping as default, choose a coarser tier when needed, return min/avg/max/count and explicit gap metadata, and reject invalid scope/ID/metric/range combinations.
- **Test / verify:** Test tier selection at every boundary, cap enforcement, archived resource history, missing buckets, query injection resistance, and index-aware execution.
- **Done when:** Browsers ask for metrics by intent rather than storage-table knowledge.

### T070 — Expose the historical metrics API contract

- **Commit:** `feat(api): add metrics history endpoint`
- **Description:** Implement authenticated `GET /api/v1/metrics` with scope, ID, metric list, from/to range, and structured series/gap response.
- **Implement:** Limit requested metric count and date span, rate-limit expensive queries, name units explicitly, return RFC 3339 points, and never zero-fill missing values.
- **Test / verify:** API-contract tests for multi-series, errors, null/gap output, authorization, query limits, and resolution disclosure.
- **Done when:** Chart clients receive a stable, bounded, self-describing historical payload.

### T071 — Connect historical charts to server and resource views

- **Commit:** `feat(web): add historical telemetry charts`
- **Description:** Add range controls (1h/6h/24h/7d/30d/custom), server and resource charts, summary statistics, annotations, loading states, resolution detail, and explicit gap rendering.
- **Implement:** Default resource CPU to host-normalized percentage; include memory/network/block I/O where meaningful; preserve inactive versus collector-failure gap semantics.
- **Test / verify:** Playwright-test every range, custom validation, chart gaps, markers, tooltip/focus summary, resolution changes, and responsive rendering.
- **Done when:** Users can correlate changes over time without misleading interpolation.

### T072 — Implement manual history deletion operations

- **Commit:** `feat(storage): add scoped history deletion jobs`
- **Description:** Add controlled operations for one-resource history deletion, delete-before-date, reset-all history, and archived-resource purge with bounded batches and progress state.
- **Implement:** Require exact scope preview and typed confirmation at API/UI layers; never delete configuration unless explicitly requested; block conflicting deletion jobs.
- **Test / verify:** Test confirmation validation, each scope boundary, cancellation/retry, progress reporting, and preservation of users/settings.
- **Done when:** Destructive data management is explicit, bounded, and auditable.

## Phase 7 — Authentication, onboarding, settings, and diagnostics

### T073 — Implement local admin credentials and Argon2id hashing

- **Commit:** `feat(auth): add single-admin credential service`
- **Description:** Implement one-local-admin creation, username/password validation, Argon2id hashing with calibrated parameters, and safe credential repository operations.
- **Implement:** Reject duplicate or invalid usernames; do not implement teams, roles, invitations, or password recovery in alpha; keep password values out of logs/errors.
- **Test / verify:** Test hash/verify, invalid credentials, duplicate bootstrap, timing-safe compare path, and password policy boundary cases.
- **Done when:** Alpha has a secure, deliberately minimal local identity model.

### T074 — Implement hashed session lifecycle and logout controls

- **Commit:** `feat(auth): add secure browser session lifecycle`
- **Description:** Add opaque random session tokens stored only as hashes, secure HttpOnly cookies, idle/absolute expiry, rotation after login, current-session logout, and logout-all.
- **Implement:** Use Secure cookies when HTTPS/proxy signals warrant it, SameSite policy appropriate to the flow, optional hashed user-agent/IP-prefix binding for diagnostics, and expiry cleanup.
- **Test / verify:** Test login rotation, expiry, revocation, logout-all, cookie flags, token non-persistence, and concurrent session handling.
- **Done when:** Browser authentication meets the alpha session requirements without exposing reusable tokens.

### T075 — Add login/setup rate limits and CSRF defenses

- **Commit:** `feat(auth): protect state-changing browser requests`
- **Description:** Add bounded rate limiting for login, setup-token attempts, diagnostics generation, and expensive metric queries; implement CSRF protection for session and settings mutations.
- **Implement:** Key limits safely by IP prefix/account where applicable, avoid unbounded attacker-controlled maps, return actionable retry responses, and integrate SameSite cookie behavior.
- **Test / verify:** Test bursts, expiry/recovery, setup-token attempts, CSRF missing/invalid/valid requests, and memory-bound behavior.
- **Done when:** Public exposure has basic abuse protection beyond cookie defaults.

### T076 — Implement secure first-run setup-token policy

- **Commit:** `feat(auth): require secure first-run setup token`
- **Description:** Implement fresh-instance state machine requiring a high-entropy operator-provided secret/environment setup token for public/non-loopback setup, with generated startup-log token allowed only in local/development mode.
- **Implement:** Expire/disable setup permanently after successful admin creation; never silently re-enable it; avoid logging supplied tokens and make exposure classification conservative.
- **Test / verify:** Test public refusal without token, local generated token path, expiration, one successful claim, replay refusal, and race between simultaneous setup attempts.
- **Done when:** An arbitrary visitor cannot claim a fresh public deployment.

### T077 — Add bootstrap credentials and encrypted UI-secret service

- **Commit:** `feat(auth): support credential bootstrap and encrypted secrets`
- **Description:** Support admin bootstrap credentials from environment/Docker secrets and add authenticated encryption for future UI-entered secrets using an externally supplied master key.
- **Implement:** Auto-create only when no user exists; store ciphertext/nonce/algorithm/key version, never return secret values via API, and fail safely when UI-secret encryption is requested without master key.
- **Test / verify:** Test bootstrap idempotency, secret encryption/decryption, wrong/missing key, API redaction, and no plaintext database persistence.
- **Done when:** Installation secrets and future integration secrets follow one defensible storage policy.

### T078 — Implement authentication API endpoints and login UI

- **Commit:** `feat(web): add login logout and session controls`
- **Description:** Expose login/logout/logout-all endpoints and build accessible login/session UI with rate-limit feedback and safe redirect behavior.
- **Implement:** Return generic credential-failure errors, rotate session on success, protect mutation endpoints with CSRF token/header flow, and avoid open redirects.
- **Test / verify:** Contract-test auth endpoints; Playwright-test login failure/success, logout, logout-all, expired session, focus/error handling, and keyboard flow.
- **Done when:** Admins can securely enter and leave the application through the real browser flow.

### T079 — Implement onboarding diagnostics service

- **Commit:** `feat(onboarding): add independent installation diagnostics`
- **Description:** Add checks for host metrics access, Docker API access, cgroup access, Compose/Coolify detection, persistent storage writability, database initialization, and informational outbound availability.
- **Implement:** Each diagnostic returns independent status, clear reason, suggested fix, and expandable technical detail; optional failures never block unrelated setup completion.
- **Test / verify:** Fake each success/failure condition, validate safe technical detail redaction, and test Docker outage while host diagnostics pass.
- **Done when:** Operators receive actionable setup failures rather than a generic broken install.

### T080 — Build the one-time onboarding flow and checklist

- **Commit:** `feat(web): add secure first-run onboarding`
- **Description:** Build setup-token verification, admin creation, exposure acknowledgement, diagnostics, sampling/retention confirmation, completion state, and dismissible post-setup checklist.
- **Implement:** Keep flow short; do not require notifications/checks; make step completion resumable until admin creation and inaccessible afterward; explain required restart settings separately.
- **Test / verify:** Playwright-test normal Coolify-style setup, failed Docker diagnostic, token failure, reload/resume, completion lockout, and mobile flow.
- **Done when:** A normal deployment reaches a useful dashboard without terminal commands.

### T081 — Implement settings storage, precedence, and PATCH API

- **Commit:** `feat(settings): persist eligible admin overrides`
- **Description:** Add settings service/repository and authenticated `GET/PATCH /api/v1/settings` for eligible values, effective source, validation, live/restart-required label, and audit attribution.
- **Implement:** Route writes through storage ownership; reject deployment-critical fields; apply live settings atomically to collectors/workers where safe and show pending restart state otherwise.
- **Test / verify:** Test precedence, validation, live interval change, rejected critical setting, concurrent writes, source labels, CSRF/auth, and restart-required response.
- **Done when:** Settings are transparent and cannot secretly override deployment security boundaries.

### T082 — Build collection, retention, appearance, and system settings UI

- **Commit:** `feat(web): add alpha settings dashboard`
- **Description:** Implement settings sections for collection, retention/storage, authentication session policy, appearance, privacy/network status, and system health with effective-source and apply-mode labels.
- **Implement:** Include aggressive-interval warnings, DB budget state, retention advanced mode, theme/density choices, and clear links to restart-required deployment action; do not expose post-alpha integrations as active controls.
- **Test / verify:** Playwright-test validation, source labels, live update, restart-required display, compact/dark/light appearance controls, and keyboard navigation.
- **Done when:** Advanced configuration remains discoverable without requiring YAML or hiding operational implications.

### T083 — Implement diagnostics preview and download bundle

- **Commit:** `feat(diagnostics): add preview-first support bundle`
- **Description:** Generate a reviewable diagnostics bundle containing version/commit, OS/architecture, schema version, collector health, sanitized configuration, recent internal errors, Docker version, counts, DB size, and self-metric summary.
- **Implement:** Preview exact fields before download; exclude passwords, tokens, secret URLs, full domains/IPs unless necessary, environment variables, app logs, and database contents; rate-limit generation.
- **Test / verify:** Test allowlist/redaction, archive content, failed collection partial bundle, generation rate limits, and download headers.
- **Done when:** Users can supply useful support data without accidental secret disclosure.

## Phase 8 — Product UI completion and self-observation

### T084 — Complete production overview behavior

- **Commit:** `feat(web): complete live operational overview`
- **Description:** Replace demo-only assumptions with production Overview composition: server health/CPU/RAM/disk, active resource groups, infrastructure, collector/persistence warnings, and recent events.
- **Implement:** Prioritize unhealthy/stale resources and critical storage state; show no alert/incident UI beyond alpha scope; maintain usable mobile first viewport.
- **Test / verify:** Playwright-test healthy, Docker-down, storage-degraded, empty-host, archived-only, dark/light, compact, and mobile scenarios.
- **Done when:** The first screen fulfills the alpha operational questions under both normal and partial-failure conditions.

### T085 — Add deployment and event annotations to charts

- **Commit:** `feat(web): annotate charts with operational events`
- **Description:** Render deployment/replacement, OOM, lifecycle, boot, collector-failure, and persistence-gap annotations on applicable host/resource charts.
- **Implement:** Keep marker density bounded, provide keyboard/focus accessible event summaries, and link markers to events/resource details; do not invent incident correlations.
- **Test / verify:** Test annotation placement at boundaries, overlap, hidden marker aggregation, gap shading, tooltip/focus behavior, and reduced motion.
- **Done when:** Charts explain notable state changes without becoming visual noise.

### T086 — Add archived-resource experience and purge UI

- **Commit:** `feat(web): manage archived resource history`
- **Description:** Complete Archived navigation/detail presentation and connect typed-confirmation purge/history-deletion flows with scope preview and progress feedback.
- **Implement:** Make archived state unmistakable, keep historical charts/events accessible until deletion, and prevent destructive controls from appearing in ordinary active-resource quick actions.
- **Test / verify:** Playwright-test archive discovery, history access, typed confirmation mismatch/success, deletion progress/error, and return to archived list.
- **Done when:** Historical cleanup is deliberate and cannot be confused with workload control.

### T087 — Build monitor-health page

- **Commit:** `feat(web): expose TALOS self-observation`
- **Description:** Implement Settings → System → Monitor health with TALOS CPU/RSS/heap/goroutines, DB/WAL sizes, queue depth, dropped batches, write latency, worker durations, collector durations, SSE clients, and Docker API health.
- **Implement:** Explain units and threshold states; show unavailable measurements honestly; link persistence/storage states to relevant settings/recovery guidance.
- **Test / verify:** Component tests for values/nulls/thresholds; Playwright-test responsive layout, accessible summaries, and live updates.
- **Done when:** Operators can determine whether TALOS itself is contributing to host load or losing history.

### T088 — Add frontend error, stale, and offline resilience states

- **Commit:** `fix(web): harden stale and disconnected UI states`
- **Description:** Standardize live-stream reconnect, API failure, stale-domain, partial-data, empty, and retry states across overview, server, resources, charts, events, and settings.
- **Implement:** Preserve last known display only when explicitly marked stale; prevent stale values from appearing current; give concise actionable error text plus technical disclosure.
- **Test / verify:** Simulate SSE loss, 401, 5xx, malformed API payload, collector outage, and recovery in unit/Playwright tests.
- **Done when:** Partial backend failure produces clear degradation rather than broken or deceptive screens.

### T089 — Complete WCAG 2.2 AA accessibility pass

- **Commit:** `test(web): enforce alpha accessibility smoke coverage`
- **Description:** Audit and remediate keyboard paths, focus visibility/order, semantic landmarks, forms, dialogs, color contrast, status text, chart summaries, and reduced-motion behavior across alpha pages.
- **Implement:** Add automated axe Playwright smoke tests and a documented manual screen-reader/keyboard checklist; fix issues in the owning components rather than suppressing rules.
- **Test / verify:** Run axe on login, onboarding, overview, server, resource detail, events, settings, and destructive dialog; execute keyboard smoke path.
- **Done when:** Alpha has measurable WCAG 2.2 AA-oriented coverage, not an untested accessibility claim.

### T090 — Add visual regression and responsive smoke coverage

- **Commit:** `test(web): add deterministic visual and mobile regression suite`
- **Description:** Use seeded demo mode to establish visual snapshots for dark/light overview, server, resource detail, events, settings, degraded state, and key mobile layouts.
- **Implement:** Stabilize time/fonts/animation/data before capture; review snapshot changes deliberately; avoid broad screenshots that hide localized defects.
- **Test / verify:** Run Playwright visual comparisons at desktop and mobile viewports and ensure reduced-motion mode is deterministic.
- **Done when:** UI regressions in density, status, chart gaps, and responsive hierarchy are caught before release.

## Phase 9 — Packaging, security, documentation, and release qualification

### T091 — Build hardened production Docker image

- **Commit:** `build(container): add hardened multi-stage production image`
- **Description:** Create a multi-stage Docker build that compiles the frontend and CGO Go binary for supported Linux architectures, runs as non-root where socket access permits, and ships a minimal runtime image.
- **Implement:** Use read-only root filesystem compatibility, writable `/var/lib/talos` only, one HTTP port, no privileged mode/capabilities, no Node runtime, and explicit CGO/SQLite runtime libraries.
- **Test / verify:** Build/run image, inspect user/files/ports, verify embedded UI, writable data volume, read-only root behavior, and image vulnerability baseline.
- **Done when:** The production image reflects the resource/security model rather than a development environment.

### T092 — Add canonical Docker Compose deployment

- **Commit:** `feat(packaging): add Docker Compose installation`
- **Description:** Add the canonical Compose definition with persistent named volume, read-only host proc/sys/os-release mounts, Docker socket, non-privileged hardening, health endpoint, documented environment variables, and recommended 128 MiB memory guardrail.
- **Implement:** Use alpha image tag policy, do not falsely mark Docker socket logically read-only, and document direct-socket versus restricted-proxy security tradeoff.
- **Test / verify:** Run compose config validation and an isolated deploy smoke test covering persistence, initial setup, host access, and Docker discovery when available.
- **Done when:** The documented Compose installation is a tested reference rather than an illustrative snippet.

### T093 — Add Coolify template and template-drift validation

- **Commit:** `feat(packaging): add Coolify one-click template`
- **Description:** Create the official Coolify service template derived from the canonical Compose deployment, including volumes, mounts, secrets/setup guidance, health check, domain exposure, resource limit advice, and socket warning.
- **Implement:** Add automated comparison/validation so template capability does not drift from canonical Compose; do not depend on undocumented Coolify internals.
- **Test / verify:** Validate template syntax and canonical fields; execute documented fresh-install acceptance procedure in a Coolify-compatible integration environment when available.
- **Done when:** Coolify is a first-class, reproducible alpha installation path.

### T094 — Add API security and input-limit hardening

- **Commit:** `feat(api): enforce alpha API safety limits`
- **Description:** Complete endpoint-level authorization, query/body limits, expensive-query rate limits, cache controls, security headers appropriate behind reverse proxies, and safe request logging.
- **Implement:** Ensure all routes except health/setup bootstrap behavior require appropriate session state; reject over-broad metric/event requests; never reflect sensitive inputs in errors.
- **Test / verify:** Security tests for auth bypass, CSRF, oversized ranges/bodies, rate limits, headers, path traversal/static serving, and log redaction.
- **Done when:** The exposed alpha HTTP surface is bounded and consistent with the security specification.

### T095 — Add dependency, license, SBOM, and container scanning gates

- **Commit:** `ci: add supply-chain quality gates`
- **Description:** Extend CI with Go/frontend dependency vulnerability checks, SPDX-compatible license review, SBOM generation, container scanning, and artifact/provenance preparation where feasible.
- **Implement:** Fail on actionable critical findings according to documented policy; keep generated SBOM/artifact handling deterministic and avoid uploading secrets or development databases.
- **Test / verify:** Exercise workflows on a clean build, inspect generated SBOM, simulate a policy failure fixture where practical, and document remediation ownership.
- **Done when:** Public alpha releases have baseline software-supply-chain evidence.

### T096 — Add release versioning, tags, and GHCR publishing workflow

- **Commit:** `ci(release): publish immutable versioned container artifacts`
- **Description:** Add semver-prerelease validation, multi-architecture image publication to GHCR, immutable exact tags, and guarded `stable`/`beta`/`edge` channel behavior.
- **Implement:** Ensure `stable` cannot target alpha/beta, automatic redeployment remains out of scope, and release jobs run only from protected intended refs/tags.
- **Test / verify:** Dry-run tag validation for alpha/beta/stable/invalid cases and inspect generated image metadata without publishing from ordinary PRs.
- **Done when:** Versioned artifacts can be released without mutable-tag ambiguity.

### T097 — Document install, upgrade, uninstall, and recovery procedures

- **Commit:** `docs(operations): add alpha installation and recovery guides`
- **Description:** Document Coolify and Compose install, secure setup token/bootstrap secret, host mounts, Docker socket risk/proxy option, configuration, update channels, migrations, uninstall, persistent-volume consequences, consistent SQLite copy, and corruption/disk-full recovery.
- **Implement:** State no built-in backups and no self-update; include supported-platform boundaries and calm operational status/recovery language.
- **Test / verify:** Walk every documented command/configuration in a clean environment and link-check docs.
- **Done when:** A user can install, update, remove, or recover TALOS without relying on undocumented tribal knowledge.

### T098 — Add performance benchmark harness and reports

- **Commit:** `perf: add reproducible alpha benchmark suite`
- **Description:** Build reproducible benchmark scenarios for 10/30/50/100 containers that measure RSS, average/p95 CPU, Docker API rate, SQLite write latency, database growth, SSE bandwidth/client, collection duration, and allocations where practical.
- **Implement:** Use deterministic demo/fake Docker fixtures for repeatability plus documented real-host validation; do not claim targets without measured environment/method/version data.
- **Test / verify:** Run reference 30-container benchmark for at least 30 minutes, publish machine/config/results, and compare against `<50 MB` RSS, `<0.5%` CPU, `<50 ms` write p95, `<10 KB/s` idle SSE goals.
- **Done when:** Performance claims are evidence-backed and regressions have a repeatable detection path.

### T099 — Validate collector metrics against reference semantics

- **Commit:** `test(collectors): add reference metric validation suite`
- **Description:** Add integration/manual qualification procedures comparing host metrics to Linux interfaces/tools, Docker metrics to `docker stats` semantics, filesystem values to statfs/df semantics, and rates to known counters.
- **Implement:** Document legitimate semantic differences, especially memory working set and CPU conventions; keep validation read-only and fixture-backed where possible.
- **Test / verify:** Execute against supported Ubuntu/Debian and Docker reference environments; retain sanitized evidence in release qualification records.
- **Done when:** Alpha metric correctness is demonstrated beyond unit formulas.

### T100 — Add alpha release checklist and final gate automation

- **Commit:** `docs(release): add alpha.1 qualification checklist`
- **Description:** Add a release checklist and CI/manual gate record covering security, migrations, Coolify/Compose fresh installs, upgrade test after a prior alpha, redeploy identity, overlap aggregation, retention, persistence failure, benchmarks, SSE, accessibility, themes, mobile, docs, license, and security policy.
- **Implement:** Mark gates with objective evidence links/commands; permit only documented minor visual defects; explicitly reject publication when critical security or normal-operation data-loss defects remain.
- **Test / verify:** Run `make check`, build/image checks, end-to-end demo/host smoke suite, and complete the release record using actual outputs.
- **Done when:** `v0.1.0-alpha.1` has a defensible, repeatable go/no-go process.

## Deferred roadmap — not alpha.1 tasks

These are intentionally **not** commit-ready work for this release. Do not implement them while completing the tasks above without an explicit amended product decision and new tasks.

| Planned release | Deferred capability |
| --- | --- |
| v0.2 | HTTP/TLS health checks, deterministic alert rules, silences, cooldown/recovery, deployment grace, alert UI |
| v0.3 | SMTP/webhook/Discord/Slack/Teams/Telegram notifications and lightweight incident grouping |
| v0.4 | Bounded Docker logs, redaction, time correlation, and demand-driven read-only process explorer |
| v0.5 | Optional read-only Coolify API enrichment, TOTP, trusted-proxy/external authentication |
| v0.6 | Read-only API tokens, CSV/JSON export, optional Prometheus endpoint, limited personalization |
| Later | Database integrations, central multi-server dashboard, outbound-only agents, additional runtimes, advisory anomaly hints |
