# Metric validation

This guide describes how TALOS collector metrics map to reference Linux and
Docker interfaces, and how to verify them.

## Host metrics

| TALOS metric | Reference interface | Validation notes |
| --- | --- | --- |
| CPU percent | `/proc/stat` aggregate `cpu` line | Busy percentage over the sampling interval; should stay in `[0, 100]`. |
| Memory used | `/proc/meminfo` `MemTotal` - `MemAvailable` | Matches the kernel's own "used" convention on modern Linux. |
| Load | `/proc/loadavg` first field | 1-minute load average. |
| Uptime | `/proc/uptime` first field | Seconds since boot. |
| Network RX/TX | `/proc/net/dev` counters | Rates are deltas over elapsed time; counters never decrease. |
| Disk used/total | `statfs` on `/proc/1/root` | Matches `df` block counts where the same mount is visible. |

## Docker metrics

| TALOS metric | Reference | Semantics |
| --- | --- | --- |
| CPU host percent | `docker stats` `CPU %` | Ratio of container CPU time to system CPU time, expressed as a percent of one host core. |
| CPU cores ratio | `docker stats` | `container_delta / system_delta`; multiply by host cores for absolute core usage. |
| Memory working set | `docker stats` `MEM USAGE` | `usage - inactive_file`, falling back to `usage` when cache data is unavailable. |
| Memory percent | `docker stats` `MEM %` | Working set divided by container limit or host memory, whichever is smaller. |
| IO rates | `docker stats` `NET I/O`, `BLOCK I/O` | Counter deltas divided by elapsed seconds. |

## Known semantic differences

- **Memory working set:** TALOS uses `usage - inactive_file`, the same convention
  as the Docker CLI. This intentionally excludes reclaimable page cache and can
  be significantly lower than raw `usage`.
- **CPU host percent:** TALOS reports percent of one host core. A container
  using two full cores on a four-core host reports `200%` in `docker stats` but
  `50%` host percent here.
- **Disk:** TALOS measures the root filesystem visible at `/proc/1/root`. This
  matches the host root on typical single-disk deployments but differs from
  per-container overlay sizes.

## Running the reference suite

The fixture-backed suite lives in `internal/collector/reference` and covers
normalization formulas plus live `/proc` sanity checks:

```bash
go test ./internal/collector/reference/...
```

For release qualification, capture the same values from the reference tools on
the target host and attach sanitized evidence to the release record.
