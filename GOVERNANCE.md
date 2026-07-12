# Binnacle Governance

## Maintainer model

Binnacle is founder-led and remains in the project owner's personal GitHub
account. The maintainer sets roadmap priorities, approves releases, and makes
final decisions on product scope, branding, and architecture.

Contributors are encouraged to propose fixes and focused features through
issues and pull requests. Copyright assignment is not required; contributions
are accepted under the repository license through DCO sign-off.

## Decision process

Routine fixes and small features use normal pull-request review. Material
changes require a lightweight proposal before implementation when they affect:

- security boundaries or Docker access;
- storage model, retention, or migration behavior;
- public API compatibility;
- licensing or governance;
- deployment model or supported platforms;
- frontend architecture or large dependencies;
- compatibility guarantees or release policy.

The default decision standard is a simple, maintainable solution that preserves
Binnacle's low-overhead, local-first, read-only scope.

## Architecture decision records

Accepted architectural decisions live in `adr/` and use sequential filenames:

```text
NNN-short-title.md
```

An ADR contains context, the decision, consequences, considered alternatives,
and links to the relevant specification section. An ADR supersedes an earlier
ADR only when it says so explicitly. The product specification remains the
source of truth unless the maintainer records an amendment.

## Releases

Releases follow semantic prerelease versions until 1.0. Exact version tags are
immutable. `stable` must not reference alpha or beta releases; `beta` may
reference beta or release-candidate builds; `edge` is development-only.
