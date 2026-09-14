# RouterForge Management v2 API

Status: **Stable 0.8.0 product topology**. The Management v2 contract introduced in 0.7.1 remains compatible.

## Security
Mutations require fixed method, same-origin, live Entware-root session, exact confirmation/whitelist where applicable and Core-injected internal Admin marker.

## Processes
`POST /api/modules/admin/processes/<pid>/signal`
Allowed: `TERM`, `HUP`, `INT`, `KILL`.

## Services
`POST /api/modules/admin/services/<id>/action`
Allowed: `start`, `stop`, `restart`.

## File Manager
See [MANAGEMENT_V2_FILES_API.md](MANAGEMENT_V2_FILES_API.md).

## Terminal / PTY
`GET /api/modules/admin/terminal/ws?mode=<entware|keenetic>&cols=<n>&rows=<n>`

Entware uses fixed `/opt/bin/sh -il`; Keenetic mode uses fixed server-resolved `ndmc`. Unknown modes are rejected.

## Network diagnostics ownership
Management no longer exposes the old `/api/modules/admin/network-tools/run` endpoint or the old `Network` tab. Active network diagnostics belong exclusively to `routerforge-network-tools`.

Management owns administration, Monitoring owns read-only telemetry, and Network Tools owns active diagnostics.

## Package mutations
Generic RouterForge/Entware package lifecycle remains owned by Core/App Center.