# RouterForge Management v2 API - Phase 8A

Status: Dev contract for process and Entware service mutations.

## Security boundary

Public mutation requests use the Core route `/api/modules/admin/...`.

Every Admin mutation requires all of the following:

1. HTTP `POST`.
2. Same-origin request when an `Origin` header is present.
3. A live RouterForge session authenticated as Entware `root`.
4. Exact target confirmation in the JSON body.
5. A fixed action/signal whitelist.
6. Core-to-module internal authorization marker on the root-only Unix socket.

The authenticated root session is mandatory for mutations even when general
RouterForge read access is configured with `auth_required=false`.

The browser cannot grant itself the internal authorization marker. Core removes
any incoming value and injects the canonical marker only after the session gate.

## Process signal

`POST /api/modules/admin/processes/<pid>/signal`

Body:

```json
{
  "signal": "TERM",
  "confirm_pid": 1234
}
```

Allowed signals in Phase 8A: `TERM`, `HUP`, `INT`, `KILL`.

PID 1, the Admin process itself, and RouterForge / legacy dns-monitor processes
are protected. RouterForge component lifecycle remains owned by App Center.

A successful response reports the process snapshot before signal delivery and a
best-effort immediate snapshot after delivery. `signal_delivered=true` means the
kernel accepted the signal; it does not promise that an asynchronous graceful
shutdown has already completed.

## Entware service action

`POST /api/modules/admin/services/<id>/action`

`id` is the exact init-script basename returned by `GET /api/modules/admin/services`
(for example `S99example`). It is never interpreted as an arbitrary filesystem path.

Body:

```json
{
  "action": "restart",
  "confirm_id": "S99example"
}
```

Allowed actions in Phase 8A: `start`, `stop`, `restart`.

The backend executes the exact `/opt/etc/init.d/<id>` file directly with one
whitelisted argument. No shell command string, extra arguments, path traversal,
or arbitrary executable path is accepted. RouterForge / legacy dns-monitor init
scripts are protected and stay under App Center lifecycle control.

## Read compatibility

Existing read endpoints remain compatible. Service objects gain an additive
`id` field containing the exact init-script basename.

Admin health reports:

- `mode: control`
- `mutation_api: true`
- `mutation_auth: root-session`
- `ui_mutations: false`

The current Management UI remains read-only in Phase 8A. Mutation controls are a
later Phase 8 step after the API/security contract has hardware evidence.

## Explicitly not in Phase 8A

- file write/delete/move/chmod APIs;
- terminal / PTY APIs;
- arbitrary shell execution;
- package mutation duplication (package lifecycle continues to use App Center).

## Phase 8E extension

File Manager is specified separately in `MANAGEMENT_V2_FILES_API.md`.

FE-001A introduces the filesystem path-security contract and tested path-resolution primitives only. It does **not** enable File Manager HTTP routes or filesystem mutations. The Phase 8A process/service mutation contract above remains unchanged.
