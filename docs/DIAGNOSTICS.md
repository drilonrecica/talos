# Logs and process diagnostics

Binnacle v0.4 adds session-only, read-only diagnostics. The Logs page reads
Docker logs for one container or logical resource. It supports 5 minute,
30 minute, and 1 hour ranges, literal search, and live follow. Messages are
redacted before search or delivery and are never stored by Binnacle.

Responses default to 500 lines and cannot exceed the configured 5,000-line or
1 MiB ceilings. A resource can contain at most 32 log components. Follow
sessions stop after 30 minutes or as soon as the browser disconnects.

Redaction covers common authorization headers, password/token/API-key
assignments, credential-bearing URLs, and private-key blocks. It is
best-effort, not a guarantee. Applications should never write secrets to logs.
Up to 16 additional Go RE2 expressions can be supplied with
`BINNACLE_LOGS_REDACTION_PATTERNS`, separated by `||`.

The Server page samples host processes only when requested. It uses two bounded
reads of the host `/proc`, serializes scans, returns 25 rows by default (100
maximum), and does not persist results. `/etc/passwd` is mounted read-only only
to display usernames; numeric UIDs are used when it is unavailable.

There are deliberately no process signals, exec, shell, renice, container
mutation, server-wide log search, indexing, or log persistence endpoints.
