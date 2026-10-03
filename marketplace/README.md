# RouterForge Registry / App Center metadata

`marketplace/registry/index.json` is the public compatibility Registry path. Core ships an embedded mirror and caches last-good remote state.

User-facing name: **App Center / Центр приложений**.

## Trust
Manifest submissions are schema-validated.

For third-party apps, the preferred `VERIFIED` model is persistent project verification: RouterForge binds trust to the publisher identity and GitHub `OWNER/REPOSITORY`, not to an app version or a specific manifest SHA256. Routine releases and card edits do not require re-verification.

`manifest_sha256` remains audit/provenance metadata. Legacy SHA-bound approvals remain supported for compatibility, while RouterForge `OFFICIAL` entries may continue using strict SHA-pinned approvals.

States include `OFFICIAL`, `VERIFIED`, `UNVERIFIED`, `CHANGED`, `BLOCKED`, `DEPRECATED`.

Project verification and lifecycle authority are separate. A `VERIFIED` badge never authorizes arbitrary install/update/remove code merely because it appears in developer-controlled metadata.

Developer quick start: [`developer-kit/README.md`](developer-kit/README.md).

## Channel transport
Stable reads Registry from `main`; Beta/Dev from `dev`.
RouterForge package lifecycle uses target-specific release-index with exact SHA256.

## Runtime Web UI discovery
App Center correlates local LISTEN sockets with PID/process/package and probes only discovered local application endpoints. No blind LAN/subnet scan. Redirect/XFO/CSP/SSRF gates remain fail closed.
