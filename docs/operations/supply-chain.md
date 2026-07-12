# Supply-chain policy

Binnacle keeps a narrow dependency set and verifies it in CI.

## Automated gates

- **Go vulnerability scan** — `govulncheck ./...` runs on every PR/push.
- **Frontend audit** — `pnpm audit --audit-level moderate` runs on every PR/push.
- **License review** — `go-licenses` checks that all Go dependencies use an allowlisted license (MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0).
- **SBOM** — `syft` generates an SPDX JSON SBOM on every push.
- **Container scan** — `trivy` scans the production image for HIGH/CRITICAL vulnerabilities on every push.

## Local targets

```bash
make vuln      # govulncheck + pnpm audit
make licenses  # go-licenses check
make sbom      # build image and generate SBOM
make scan      # build image and trivy scan
```

These targets require the corresponding tools (`go-licenses`, `syft`, `trivy`) to be installed locally. CI installs them automatically.

## Response

- A critical or exploitable vulnerability in a production dependency blocks release until remediated or documented as a false positive.
- License findings outside the allowlist require an ADR and replacement of the dependency.
- SBOMs and scan results are retained as release qualification evidence.
