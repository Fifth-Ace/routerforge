# RouterForge Management v2 API

Status: **Stable 0.9.0 product topology**. The Management v2 contract introduced in 0.7.1
remains compatible; Admin package version for this Stable is **0.8.1**.

## Security

Mutations require fixed method and same-origin.

- If RouterForge authentication is disabled, normal same-origin guarded Management actions are allowed.
- If RouterForge authentication is enabled, mutations require an authenticated Entware-root session.
- Cross-origin mutation requests are rejected.
- Exact confirmation/whitelist, process/path/service guards and the Core-injected internal Admin marker remain mandatory where applicable.
- Browser-supplied internal authorization markers are stripped; only Core injects the canonical marker after policy checks.

## Processes
`POST /api/modules/admin/processes/<pid>/signal`

Allowed: `TERM`, `HUP`, `INT`, `KILL`.

PID 1, RouterForge processes and other protected targets remain guarded.

## Services
`POST /api/modules/admin/services/<id>/action`

Allowed: `start`, `stop`, `restart`.

## File Manager
See [MANAGEMENT_V2_FILES_API.md](MANAGEMENT_V2_FILES_API.md).

File/config mutation paths use shared safety primitives where applicable while preserving
the existing path boundary and explicit destructive confirmations.

## Terminal / PTY
`GET /api/modules/admin/terminal/ws?mode=<entware|keenetic>&cols=<n>&rows=<n>`

Entware uses fixed `/opt/bin/sh -il`; Keenetic mode uses fixed server-resolved `ndmc`.
Unknown modes are rejected.

## Network diagnostics ownership

Management does not expose the old `/api/modules/admin/network-tools/run` endpoint or
the old `Network` tab. Active network diagnostics belong exclusively to
`routerforge-network-tools`.

Management owns administration, Monitoring owns read-only telemetry, and Network Tools
owns active diagnostics.

## Package mutations

Generic RouterForge/Entware package lifecycle remains owned by Core/App Center.
