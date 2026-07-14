# Binnacle roadmap

This roadmap communicates direction, not a binding commitment. Ordering and
scope may change based on security risk, maintenance cost, and user demand.
Detailed feature specifications belong in release planning, not here.

| Release | Status | Direction |
| --- | --- | --- |
| v0.1 foundation | Implemented | Single-server host and Docker monitoring, local history, Coolify/Compose-aware resources, authentication, and bounded operation. |
| v0.2 checks and alerts | Implemented; tag not published | HTTP/HTTPS checks, deterministic local alerts, timed silences, deployment grace, and effective resource health. |
| v0.3 notifications and incident grouping | Implemented; tag not published | Durable HTTPS webhook and SMTP delivery with automatic target-based incidents. |
| v0.4 diagnostics | Implemented; local qualification complete, tag not published | Bounded log access and read-only process diagnostics. |
| v0.5 access and Coolify enrichment | Planned | Coolify API enrichment, TOTP, and external authentication. |
| v0.6 portability and integration | Planned | API tokens, export, an optional Prometheus endpoint, and limited personalization. |

## Later direction

Potential later work includes database integrations, a multi-server dashboard,
outbound agents, additional container runtimes, and explicitly advisory anomaly
hints. These items are exploratory and have no promised release.
