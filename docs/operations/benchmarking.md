# Benchmarking Binnacle

This document describes how to run the reproducible alpha benchmark harness and
interpret the results.

## Quick start

```bash
# Short validation run (60 seconds, 30 synthetic containers)
make benchmark

# Full 10/30/50/100 container matrix (about 4 minutes with default duration)
make benchmark-matrix

# Custom run
python3 scripts/benchmark.py --containers 100 --duration 300 --output report.json
```

## What is measured

The harness starts `binnacle` in deterministic demo mode (`--demo --demo-seed 1`)
with a configurable number of synthetic containers and collects:

| Metric | Source | Purpose |
| --- | --- | --- |
| RSS | `/proc/<pid>/stat` | Resident memory footprint |
| CPU | `/proc/<pid>/stat` | Process CPU time over the run |
| SQLite write latency | `/api/v1/monitor-health` | Persistence latency |
| Queue depth / dropped batches | `/api/v1/monitor-health` | Persistence pressure |
| Go heap / goroutines | `/api/v1/monitor-health` | Go runtime pressure |
| Database size | Filesystem + monitor endpoint | On-disk growth |
| SSE bandwidth | `/api/v1/live` | Idle live-stream data rate |

Docker API rate and collection duration are meaningful only when running
against a real Docker engine; the demo harness provides a repeatable baseline
for regression detection and leaves real-host validation to release hardware.

## Alpha goals

These targets are guidance, not guarantees, and must be validated on the
release machine with the exact binary version:

- RSS `< 50 MB`
- CPU `< 0.5%` of one core
- SQLite write p95 `< 50 ms`
- Idle SSE `< 10 KB/s`

## Reproducibility

- The harness always uses seed `1`.
- Each run writes a fresh temporary `BINNACLE_DATA_DIR`.
- Reports are JSON so they can be diffed across commits.

## Real-host validation

To benchmark against real Docker and host interfaces, run `binnacle` in non-demo
mode with an isolated data directory and direct the harness at the same
`BINNACLE_LISTEN_ADDRESS`. Document the host architecture, Docker version, and
binary version alongside the report.
