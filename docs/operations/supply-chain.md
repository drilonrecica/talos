# Supply-chain policy

Binnacle keeps a narrow dependency set and verifies it in CI.

## Automated gates

- **Go vulnerability scan** — `scripts/govulncheck.sh` runs on every PR/push
  and applies only the documented IDs in `.govulncheck-ignore`.
- **Frontend audit** — `pnpm audit --audit-level moderate` runs on every PR/push.
- **License review** — `go-licenses` checks Go dependencies against the
  repository's explicit license allowlist.
- **SBOM** — Anchore's SBOM action generates SPDX JSON inventories for Binnacle and the pinned socket-proxy image on every push.
- **Container scan** — `trivy` scans the production and socket-proxy images for HIGH/CRITICAL vulnerabilities on every push.

## Local targets

```bash
make vuln      # govulncheck + pnpm audit
make licenses  # go-licenses check
make sbom      # build image and generate SBOM
make scan      # build image and trivy scan
```

These targets require the corresponding local tools (`go-licenses`, `syft`,
`trivy`). CI performs equivalent checks using pinned workflow tools and actions.

`make vuln`, CI, and the release gate all invoke the same
`scripts/govulncheck.sh` accepted-risk wrapper. Run raw `govulncheck ./...`
separately when auditing the complete scanner output.

## Closed Docker/Moby findings

Reviewed and closed on 2026-07-15. Code search found calls only to container
list, inspect, one-shot stats, logs, events, and server-version reads. The
adapter stores the SDK behind an unexported interface limited to those methods;
no archive, copy, plugin, authorization-management, or mutation method is in
Binnacle's compile-time boundary.

| Finding | Affected daemon mutation path | Binnacle evidence and prior acceptance rationale | Closure |
| --- | --- | --- | --- |
| `GO-2026-4883` | Plugin privilege validation | No plugin API calls; monitoring uses only the read methods above. Previously accepted as daemon-side and unreachable. | Split Moby modules plus mandatory Engine 29.5.1+; ignore removed. |
| `GO-2026-4887` | AuthZ plugin enforcement bypass | No AuthZ management or mutation calls; Binnacle does not configure authorization plugins. Previously accepted as daemon-side and unreachable. | Split Moby modules plus mandatory Engine 29.5.1+; ignore removed. |
| `GO-2026-5617` | Container archive / `docker cp` | No archive upload/download or copy calls. Previously accepted because Binnacle only reads monitoring endpoints. | Split Moby modules plus mandatory Engine 29.5.1+; ignore removed. |
| `GO-2026-5668` | Container archive / `docker cp` | No archive upload/download or copy calls. Previously accepted because Binnacle only reads monitoring endpoints. | Split Moby modules plus mandatory Engine 29.5.1+; ignore removed. |
| `GO-2026-5746` | Container archive / `docker cp` | No archive upload/download or copy calls. Previously accepted because Binnacle only reads monitoring endpoints. | Split Moby modules plus mandatory Engine 29.5.1+; ignore removed. |

## Response

- A critical or exploitable vulnerability in a production dependency blocks release until remediated or documented as a false positive.
- License findings outside the allowlist require explicit review. Incompatible
  dependencies must be replaced; an architectural licensing decision requires
  an ADR.
- SBOMs and scan results are retained as release qualification evidence.
