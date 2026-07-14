# ADR 017: Bounded read-only diagnostics

Status: Accepted for v0.4

## Decision

Binnacle may read container logs and host process metadata on demand. These
interfaces remain session-authenticated, read-only, bounded, cancellable, and
ephemeral. Log content is redacted before search or delivery and is never
stored. Process data is sampled from the mounted host `/proc` for each request
and is never retained.

The Docker boundary exposes only bounded log reads. It does not expose exec,
signals, shell access, container mutation, or daemon-wide log search. Process
diagnostics expose no signal, renice, or mutation operation.

## Limits

- log responses default to 500 lines and are capped at 5,000 lines and 1 MiB;
- a logical resource expands to at most 32 containers;
- live follows end after 30 minutes and stop on client cancellation;
- custom redaction accepts at most 16 valid RE2 expressions;
- process scans are serialized, use two bounded samples, and return at most
  100 rows (25 by default).

Redaction is best-effort. Operators must still treat diagnostic output as
sensitive and avoid placing secrets in application logs.

## Consequences

Diagnostics remain useful during incidents without turning Binnacle into a
general remote administration or log-retention system. Bounded reads can omit
older data and redaction cannot recognize every secret format; the UI and API
therefore report truncation and the best-effort redaction boundary explicitly.
