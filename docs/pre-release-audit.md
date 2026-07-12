# TALOS v0.1.0-alpha.1 Pre-Release Audit

**Audited:** 2026-07-12  
**Scope:** Conformance of the implemented codebase against `docs/SPEC.md` and `docs/TASKS.md` for the `v0.1.0-alpha.1` release.  
**Methodology:** Read-only review of all specification and task documents, automated exploration of the repository, manual source inspection, and execution of the existing Go/Vitest test suites, `make check`, and `make build`.

## Executive summary

The repository implements a large portion of the alpha scope, but `docs/TASKS.md` incorrectly marks many work items as **Complete** while the corresponding code is incomplete or deviates from the specification. This audit documents the gaps found, the fixes applied, and the remaining pre-release risks.

After the fixes in this audit:

- `make check` passes (Go tests, vet, formatting, frontend type check, lint, unit tests).
- `make build` produces a working CGO-enabled binary with embedded frontend assets.
- The demo binary starts and `/healthz` responds correctly.
- Authentication is now required even in demo mode.

A second pass closed the high-severity schema and persistence gaps (host metrics, filesystems, container instances, events) and added the missing historical events UI. The remaining gaps are medium or low severity and are documented in [Remaining gaps](#remaining-gaps).

## Fixes applied

| # | Severity | Finding | Spec / Task reference | Fix |
|---|----------|---------|----------------------|-----|
| 1 | Critical | Health check in `Dockerfile` and `docker-compose.yml` hit `GET /api/v1/session`, which requires a valid session and returns 401 on a fresh install. Container is marked unhealthy before first login. | SPEC §8.2, §44.1 | Added `--healthcheck` CLI flag that performs a one-shot HTTP GET to the existing unauthenticated `/healthz` endpoint. Updated Dockerfile and Compose health checks to use `talos --healthcheck`. |
| 2 | Critical | Docker Compose set `user: talos` but the `talos` user is not in the host `docker` group, so the Docker collector is denied socket access by default. | SPEC §8.2 | Changed Compose to `user: "talos:${DOCKER_GID}"` and documented the required `export DOCKER_GID=$(getent group docker \| cut -d: -f3)`. |
| 3 | Critical | Demo mode used `DemoAuthorizer`, which authorized every request. Accidental `TALOS_DEMO=true` in production granted full unauthenticated admin access. | SPEC §37, §46 | Refactored `cmd/talos/main.go` so auth, setup, onboarding, and settings services are initialized in demo mode too. Demo mode now only replaces real host/Docker collection with synthetic data; authentication is still required. |
| 4 | High | Generated local setup token was logged as a structured field, exposing the bootstrap secret in process logs. | SPEC §14.2, §15 | Removed the token value from logs. The warning now says the token is available through the setup UI. |
| 5 | High | Persistence queue overflow logic was confusing and could double-count dropped batches. | SPEC §22.5 | Simplified `storage.Persistence.enqueue` to drop the oldest batch once and retry; clarified accounting. |
| 6 | High | `events` inserts hard-coded `severity='info'` and `source='talos'` and omitted `host_id`, `details_json`, and `correlation_key`. | SPEC §23.10 | Updated `storage/writer.go` to derive severity from event type, set `host_id`, use a type-appropriate source, and populate `details_json` when available. |
| 7 | High | `collector_state_events` table was created but never written, breaking gap classification in historical metrics. | SPEC §23.11 | `WriteBatch` now tracks previous collector states in `Manager.prevCollectors` and writes `collector_state_events` rows on every state transition. |
| 8 | Medium | `production.Sampler` called `Docker.Inspect` for every listed container on every tick, and ignored stopped containers. | SPEC §18.2, §18.9, §19.7 | Integrated the existing Docker metadata cache (`internal/collector/docker/cache.go`) with lifecycle-event updates; non-running containers are now included in resource inventory with correct status, while stats collection still runs only for running containers. |
| 9 | Medium | `docker.max_concurrency` was parsed but never enforced. `dockerapi.Limited` existed but had no delegating methods. | SPEC §13.3 | Completed `dockerapi.Limited` with bounded `List`, `Inspect`, `Stats`, `Version`, `Diagnostics`, and `Close` methods; wrapped the engine with it in `main.go`. |
| 10 | Medium | Retention presets and database budget thresholds were parsed but never acted upon. Database growth was unbounded. | SPEC §24, §25.2, §39 | Added `internal/storage/retention.go` with an hourly retention worker that applies raw/1m/15m/1h cutoffs, evaluates the database+WAL size every minute, exposes `BudgetState`, and pauses raw persistence at the emergency threshold. Persistence checks `Manager.EmergencyPause()` before enqueueing. |
| 11 | Medium | `make dev`, `make dev-demo`, and `make dev-host` were stubs that exited with error. | SPEC §44.1, TASKS T005 | Replaced stubs with working targets that build the binary and run it with a fresh temporary `TALOS_DATA_DIR` and a loopback listen address. `dev` also starts the Vite dev server. |
| 12 | Low | `.github/workflows/docs.yml` link check was a no-op (`\| true`). | SPEC §44, §45 | Replaced the shell pipeline with a Python script that validates relative `.md` links in `docs/` and `adr/`. |
| 13 | Low | `CODE_OF_CONDUCT.md` was referenced in `README.md` but did not exist. | SPEC §6.1, §11 | Added a standard Contributor Covenant code of conduct. |
| 14 | Low | `settings.Defaults()` hard-coded `RuntimeDir` to `/var/lib/talos/runtime`, overriding `TALOS_DATA_DIR`. | SPEC §13.2 | Removed the default runtime dir and made `Normalize()` derive it from `DataDir` when unset, so `TALOS_DATA_DIR` affects all paths. |
| 15 | High | `container_instance_samples_10s` table existed but was never written; `container_instances` rows were also missing. | SPEC §23.7, TASKS T055/T058 | Extended `metrics.ResourceComponent` with CPU, memory, network, block I/O, and PIDs. `WriteBatch` now upserts `container_instances` and writes per-component samples to `container_instance_samples_10s`. Added migration `013_broaden_instance_telemetry.sql`. |
| 16 | High | `host_samples_10s` schema only stored CPU, memory, network, and block BPS. | SPEC §17, §23.5, TASKS T026–T030 | Broadened `HostObservation` with per-mode CPU, load 1/5/15, memory available/cached/buffers, swap, disk IOPS, and uptime. Added migration `012_broaden_host_telemetry.sql` and updated `WriteBatch`. |
| 17 | High | Host disk usage used `statfs` on the container root; real host filesystems were not persisted. | SPEC §17.8, TASKS T030 | Implemented `collectFilesystems` by parsing `/proc/self/mountinfo`, filtering pseudo-filesystems, and stat-ing each mount. Added `FilesystemObservation` and persisted to `filesystem_samples_1m` via `Engine.PublishFilesystems`. |
| 18 | Medium | `metrics.Event` lacked `Severity`, `Details`, `CorrelationKey`, and `ContainerInstanceID`. | SPEC §23.10, §55.4 | Extended `metrics.Event` and `HistoricalEvent`; updated `WriteBatch`, `WriteEvent`, Docker normalizer, demo generator, and retention events to populate the new fields. |
| 19 | Medium | Frontend `Events.svelte` only rendered live SSE events. | SPEC §28.6, TASKS T062/T063 | Replaced the live-only view with an event-history page that queries `GET /api/v1/events` for the last 1h/6h/24h/7d and lists severity, type, summary, source, and metadata. Added `web/src/lib/events.ts` and unit tests. |
| 20 | Medium | Go toolchain was pinned to an unreleased `1.26.4` local build. | SPEC §9.1, §44 | Pinned the project to Go `1.24.4` (latest public stable) in `go.mod` and CI workflows. |
| 21 | Low | Migrations lived under `internal/storage/migrations`, deviating from the repository layout. | SPEC §11 | Moved migrations to root `migrations/` and exposed them through `migrations/embed.go` so the storage package can embed them. |

## Remaining gaps

The following items were identified but not fixed in this pass because they require larger schema, data-model, or UI work. They should be addressed before declaring alpha.1 release-ready.

| # | Severity | Finding | Spec / Task reference | Recommended action |
|---|---|----------|---------|----------------------|--------------------|
| R1 | Medium | Rollup tables only aggregate CPU, memory, network, and block I/O. Disk, load, PIDs, swap, and per-interface metrics are not rolled up. | SPEC §24.3 | Add columns to rollup tables and update `RollupOnce` once the host schema is broadened. |
| R2 | Low | Frontend unit tests were expanded but Playwright e2e smoke tests are still not wired into the executed suite. | SPEC §45.5, TASKS T089/T090 | Add Playwright smoke tests for login, overview, server, resource detail, settings, and the new event history page. |
| R3 | Low | No performance benchmark harness is wired into CI. The `benchmark-report.json` at the repository root appears stale. | SPEC §39.3, TASKS T098 | Implement reproducible benchmark scenarios and keep the report current with measured environment details. |
| R4 | Low | `container_instance_samples_10s` recommends `cpu_core_equiv` and `memory_usage_bytes`; these columns exist but are left NULL because per-component online-CPU and raw usage are not currently collected. | SPEC §23.7 | Capture container online CPUs and raw memory usage in `ResourceComponent` if instance-level detail is required. |

## Verification

After applying the fixes above, the following commands completed successfully in the development environment:

```text
$ make check
$ make build
$ go test ./...
$ cd web && pnpm check && pnpm lint && pnpm format && pnpm test:run
```

A manual smoke test confirmed:

1. `./bin/talos --demo --demo-seed 42 --demo-containers 5` starts with `TALOS_DATA_DIR`, `TALOS_LISTEN_ADDRESS=127.0.0.1:8080`, and `TALOS_SETUP_TOKEN` set.
2. `./bin/talos --healthcheck` returns exit code 0 against the running instance.
3. `GET /api/v1/server` returns HTTP 401 when unauthenticated (correct behavior after the demo-auth fix).
4. `docker compose -f packaging/docker/docker-compose.yml config` validates syntax when `DOCKER_GID` and `TALOS_SETUP_TOKEN` are supplied.
5. The `/events` page loads historical events from `GET /api/v1/events` and renders severity badges.

## Conclusion

The project is now **substantially closer** to `docs/SPEC.md` for `v0.1.0-alpha.1`. The fixes in this audit remove the critical and high-severity security, deployment, data-integrity, and schema-conformance blockers identified in the first pass. The remaining gaps are medium or low severity and are limited to rollup completeness, e2e test coverage, benchmark automation, and optional instance-level detail columns. Before release, resolve R1 (rollups) and add at least basic Playwright smoke tests.
