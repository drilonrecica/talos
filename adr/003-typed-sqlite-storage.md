# 003 — Typed SQLite storage

## Context

Historical metrics must remain local and queryable.

## Decision

Use typed SQLite tables, not EAV storage. See [SPEC §23](../docs/SPEC.md#23-sqlite-storage-architecture).

## Consequences

Schema migrations are owned by Binnacle.

## Alternatives

External databases and generic metric-value rows are rejected.
