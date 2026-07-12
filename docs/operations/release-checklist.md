# Alpha.1 release checklist

This checklist covers the objective gates required before publishing
`v0.1.0-alpha.1`.

## Automated gate

Run the full release gate:

```bash
./scripts/release-gate.sh
```

It produces `release-record/v0.1.0-alpha.1-<short-sha>.md` with a pass/fail
table and captured benchmark output.

## Required gates

| Gate | Why it matters | Reject if |
| --- | --- | --- |
| `make check` | Local CI-quality subset (format, vet, tests, lint) | Any check fails |
| License and security policy | Legal and responsible disclosure baseline | `LICENSE` or `SECURITY.md` missing |
| Binary build | Production artifact compiles | Build error |
| Container image build | Installation artifacts exist | Image build fails |
| Demo container smoke | Core API responds from the published image | `/api/v1/session` fails |
| Benchmark | Performance regressions detected | RSS, CPU, write latency, or SSE exceed documented goals on reference hardware |

## Optional gates

| Gate | Notes |
| --- | --- |
| E2E accessibility | `pnpm --dir web test:e2e a11y.spec.ts` |
| E2E visual regression | `pnpm --dir web test:e2e:visual` |
| Supply-chain scan | `make vuln` (requires network and `govulncheck`) |
| Real-host validation | Run `binnacle` against Docker and compare metrics to `docker stats` / `/proc` |
| Coolify fresh install | Deploy `packaging/coolify/binnacle.yaml` to a Coolify instance |
| Compose fresh install | `docker compose -f packaging/docker/docker-compose.yml up` |
| Upgrade test | Install previous build, persist data, upgrade to candidate |
| Retention / persistence failure | Fill queue, verify drops are bounded and data recovers |

## Go/no-go rules

- **GO:** All required gates pass. Optional gates may be skipped only with a
  documented reason. Minor visual defects are acceptable if recorded.
- **NO-GO:** Any critical security defect, normal-operation data loss, or
  required gate failure remains.

## Evidence retention

Attach the following to the release record:

1. `release-record/build.log`
2. `release-record/v0.1.0-alpha.1-<short-sha>.md`
3. `benchmark-report.json`
4. Container image digest (`docker inspect --format='{{index .RepoDigests 0}}' ghcr.io/drilonrecica/binnacle:local`)
5. E2E and visual regression reports when run
