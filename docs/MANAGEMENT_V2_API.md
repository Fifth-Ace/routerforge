# RouterForge Management v2 API

Status: **Stable 0.7.1 contract**.

## Security
Mutations require fixed method, same-origin, live Entware-root session, exact confirmation/whitelist where applicable and Core-injected internal Admin marker. Browser cannot authorize itself directly.

## Processes
`POST /api/modules/admin/processes/<pid>/signal`
Allowed: `TERM`, `HUP`, `INT`, `KILL`.

## Services
`POST /api/modules/admin/services/<id>/action`
Allowed: `start`, `stop`, `restart`.
Backend executes exact `/opt/etc/init.d/<id>` with one whitelisted argument.

## File Manager
See [MANAGEMENT_V2_FILES_API.md](MANAGEMENT_V2_FILES_API.md). UI is enabled in Stable 0.7.1.

## Terminal / PTY

```text
GET /api/modules/admin/terminal/ws?mode=<entware|keenetic>&cols=<n>&rows=<n>
```

Entware:
- fixed `/opt/bin/sh -il`;
- default cwd `/opt`.

Keenetic:
- fixed server-resolved `ndmc`;
- fixed cwd `/opt`;
- no request-controlled executable/argv.

Unknown mode rejected. Both modes and switching were hardware-validated on Keenetic Ultra KN-1812.

REST terminal compatibility remains Entware-oriented; interactive UI uses WebSocket/PTTY.

## Package mutations
Generic RouterForge/Entware package lifecycle remains owned by Core/App Center.
