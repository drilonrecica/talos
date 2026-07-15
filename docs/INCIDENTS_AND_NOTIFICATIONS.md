# Incidents and notifications

Incidents automatically group overlapping firing alerts for one affected
target. The first alert opens an incident, later alerts join it, and critical
members promote its severity. An incident resolves after every member alert
resolves. A later alert opens a new incident; acknowledgement, assignment, and
manual resolution are intentionally outside v0.3.

On startup, firing alerts without incident membership are reconciled without
sending historical notifications. Resolved incidents are retained for one
year; completed delivery records are retained for 90 days.

## Channels

Up to 32 channels may be configured under Alerts → Channels. HTTPS webhooks
support an optional bearer token and HMAC-SHA256 signature. SMTP requires
STARTTLS or implicit TLS, one sender, and 1–20 recipients.

Set `BINNACLE_MASTER_KEY_FILE` before creating channels and point it to a
Docker/Coolify secret containing a raw 32-byte value, a base64 encoding of 32
bytes, or a 64-character hexadecimal key. The legacy `BINNACLE_MASTER_KEY`
environment variable accepts the same formats, but must not be configured at
the same time. If neither is present, monitoring and incidents continue, but
channel creation and delivery report `master_key_missing`.

New channels receive only later updates or reminders. Open and update events
wait 15 seconds to coalesce simultaneous alerts. If an incident resolves
before its opening delivery is attempted, the pending lifecycle delivery is
cancelled as transient noise. Open incidents receive a reminder every two
hours.

Webhook payloads use schema version 1 and include the event type, delivery and
idempotency identifiers, incident lifecycle state, affected target, alert
counts, and up to 20 member alerts. The stable idempotency key is also sent in
the `Idempotency-Key` header. When an HMAC secret is configured,
`X-Binnacle-Signature` contains `sha256=<hex digest>` over the exact request
body. Email is plain text and uses the same stable key in `Message-ID`.

The initial attempt is followed by retries after 1 minute, 5 minutes, 15
minutes, 1 hour, 4 hours, and 12 hours. Webhook 408/425/429/5xx and network,
DNS, or TLS failures retry. SMTP 4xx and network failures retry; SMTP 5xx and
invalid configuration fail permanently. Manual retry preserves the stable
idempotency key.

## Outbound security and limits

DNS is checked at configuration and dial time. Environment proxies and webhook
redirects are disabled. Loopback, link-local, multicast, unspecified,
`.localhost`, and cloud metadata targets are always blocked. Private targets
require `BINNACLE_NOTIFICATIONS_ALLOW_PRIVATE_TARGETS=true` and a restart.

| Environment variable | Default |
| --- | --- |
| `BINNACLE_NOTIFICATIONS_MAX_CONCURRENCY` | `4` |
| `BINNACLE_NOTIFICATIONS_QUEUE_CAPACITY` | `1000` |
| `BINNACLE_NOTIFICATIONS_DELIVERY_TIMEOUT` | `15s` |
| `BINNACLE_NOTIFICATIONS_REMINDER_INTERVAL` | `2h` |

Monitor Health exposes queue depth, permanent failures, overflow deferrals,
and the last successful delivery. APIs, logs, and diagnostics never expose
destinations, credentials, recipients, decrypted secrets, or message bodies.
