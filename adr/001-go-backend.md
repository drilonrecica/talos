# 001 — Go backend

## Context

Binnacle needs a small, single-process Linux service.

## Decision

Use Go for the backend. See [SPEC §10](../docs/SPEC.md#10-high-level-architecture).

## Consequences

Static deployment and standard-library HTTP are preferred; CGO is accepted for SQLite.

## Alternatives

Node.js and multi-service runtimes add operational dependencies.
