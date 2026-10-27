# RouterForge DNS Router R7 — DPR2-B (experimental)

## Contract

- Independent experimental package `routerforge-dns-router`; **no dependency on routerforge-dns**.
- No DNS listener on :53, no forwarding or interception. No service auto-start or post-install hooks.
- Offline deterministic policy engine from DPR2-A; supports max 10,000 rules and upstream IDs over UDP/TCP only (transport not used yet).
- Optional manual management-only Unix socket `/opt/var/run/routerforge-dns-router.sock` via `--serve`. Socket mode `0600`; no TCP listener.
- Frontend assets and module manifest packed for future App Center integration; visibility/auto-registration is NOT claimed.
- `GET /v1/status`, `GET/PUT /v1/config`, `POST /v1/evaluate`, `POST /v1/import-preview`, `GET /v1/export`, `GET /ui/index.html`.
- `PUT` requires `X-RouterForge-Action: save` and `If-Match` revision. Conflict `409`; configuration atomic save + directory fsync + `0600`.
- Import preview returns new candidate config but never persists it. User must save it through PUT. Rules on protected/local names rejected.
- Config format v1 is the same as DPR2-A. Config file default `/opt/etc/routerforge/dns-router/config.json`.
- HTML/CSS/JS standalone UI has no Node or external library runtime dependencies. No hidden remote calls.

## Test gates

- `go test -race ./modules/dns-router-runtime/...`, `go vet`, 10,000 rule test, CAS concurrency, mutation/header checking, protected domain rejection, import preview/no mutation and UI allowlist.
- Three cross-compiled experimental IPKs with verified control metadata, UI and manifest files. Do not add to Stable/Beta manifests.
- Exact branch SHA and GitHub Actions workflow run, `gh run watch --exit-status`.
- Protected main/dev/release refs unchanged; no router access, no installation.

## Limitations before DPR2-C

- The engine only chooses the DNS upstream; **does not send DNS requests**.
- New module is not installed or automatically listed in the public App Center. That integration is a separately tested milestone.
- DoT/DoH, runtime measurements, NAT/firewall/NDM activation, cancellation/recovery, and production lifecycle are later work.
- The UI requires a running manual socket service and authenticated Core integration (not provided in Stage B). It cannot be accessed directly by a normal browser through a Unix socket.
