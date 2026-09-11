# RouterForge — Центр приложений

Центр приложений объединяет RouterForge, Integrations, Entware, Installed и Updates.

Имена `marketplace/` и `/api/catalog*` сохраняются как compatibility surface.

## Sources of truth
Installed state → local `opkg`.
Available RouterForge state → target-specific channel release-index с exact version/asset/URL/SHA256/min-core/dependencies/conflicts.

## Lifecycle
Non-Core actions: preflight, global package-manager lock, bounded output, timeout/cancel, SSE, post-refresh, post-verify, bounded history. Core self-update restart-aware и идёт последним в batch.

## Registry trust
`OFFICIAL`, `VERIFIED`, `UNVERIFIED`, `CHANGED`, `BLOCKED`, `DEPRECATED`.
Manifest не является arbitrary shell script.

## Generic Runtime Web UI Discovery
Pipeline: `LISTEN → PID/process → package → bounded local HTTP/HTTPS probe`.

Нет blind LAN scan. Platform/infrastructure servers фильтруются; known integrations дедуплицируются; redirect/XFO/CSP/SSRF checks fail closed.

## Refresh
`GET /api/catalog` — cached read.
`POST /api/catalog/refresh` — same-origin, singleflight, rate-limited force refresh.

## Channels
Stable Registry → `main`, release → `routerforge-stable`.
Beta Registry → `dev`, release → `routerforge-beta`.
