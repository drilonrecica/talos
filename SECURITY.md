# Security Policy

## Reporting a vulnerability

Do not report suspected security vulnerabilities in public issues,
discussions, or pull requests.

Email **drilonrecica.dev@gmail.com** with a concise description, affected
version or commit, reproduction steps, impact, and any mitigation you know.
Use encrypted email if you need to share sensitive material and have a trusted
key for the maintainer.

The maintainer will acknowledge a valid report within seven calendar days and
will provide status updates while a fix is being evaluated. Please allow time
for a coordinated fix before public disclosure.

## Supported versions

Before the first stable release, only the latest published alpha or beta is
supported on a best-effort basis. Unsupported development commits may receive
a fix, but no compatibility promise is made.

## Security boundaries

Binnacle observes Docker and host state. It must not mutate monitored workloads,
proxy arbitrary Docker API calls, or expose shell access. A Docker Unix socket
is highly privileged even if its filesystem mount is read-only; operators
should prefer a constrained read-only socket proxy when their deployment model
allows it.

Binnacle sends no telemetry by default. Secrets entered through future settings
surfaces must be encrypted at rest using an operator-supplied master key and
must never be returned through an API.

## Dependency policy

Dependencies must be maintained, license-compatible with AGPL-3.0-only, and
kept to the minimum needed for the product. CI will add vulnerability, SBOM,
and license checks during alpha implementation.
