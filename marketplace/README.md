# RouterForge Registry / App Center metadata

`marketplace/registry/index.json` is the public compatibility Registry path. Core ships an embedded mirror and caches last-good remote state.

User-facing name: **App Center / Центр приложений**.

## Trust
Manifest submissions are schema-validated; approvals pin reviewed SHA256. States include `OFFICIAL`, `VERIFIED`, `UNVERIFIED`, `CHANGED`, `BLOCKED`, `DEPRECATED`.

Registry metadata never authorizes arbitrary shell commands merely because a string/URL exists.

## Channel transport
Stable reads Registry from `main`; Beta/Dev from `dev`.
RouterForge package lifecycle uses target-specific release-index with exact SHA256.

## Runtime Web UI discovery
App Center correlates local LISTEN sockets with PID/process/package and probes only discovered local application endpoints. No blind LAN/subnet scan. Redirect/XFO/CSP/SSRF gates remain fail closed.
