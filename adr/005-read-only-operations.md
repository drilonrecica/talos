# 005 — Permanently read-only operational model

## Context

Monitoring credentials are high-risk.

## Decision

Binnacle observes; it does not mutate workloads. See [SPEC §6](../docs/SPEC.md#6-product-principles).

## Consequences

No Docker proxy or control actions are added.

## Alternatives

Remediation and deployment control are rejected.
