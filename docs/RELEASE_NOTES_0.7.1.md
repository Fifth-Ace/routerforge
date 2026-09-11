# RouterForge 0.7.1 — Release Notes

Дата релиза: 2026-09-11

RouterForge 0.7.1 — большой Stable-релиз 0.7 train: consolidated Monitoring, полноценный Management v2, File Manager, Entware Terminal, Keenetic NDM Console, усиленный DNS runtime, зрелый App Center и exact-SHA release pipeline.

## Management v2

Stable 0.7.1 переводит Management из backend-заготовки в полноценную capability.

### Processes / Services
- process signals `TERM/HUP/INT/KILL` через root-session-protected contract;
- Entware service `start/stop/restart`;
- exact target confirmation и whitelist;
- Core-injected internal marker;
- никакой произвольной shell-строки.

### File Manager
UI включает Commander/Explorer, directory tree, volumes, UTF-8 editor, create, rename/move, non-recursive delete, download, properties и visual chmod.

Filesystem boundary:
- approved roots `/opt` и `/tmp`;
- explicit `..` rejected;
- canonical symlink/path containment;
- destructive operation revalidates target;
- edits use size/mtime preconditions and atomic same-filesystem replacement.

Сознательно не заявлены готовыми: recursive directory copy/delete, arbitrary binary upload, archive/extract, chown и arbitrary system-root mode.

### Terminal
Entware Terminal — WebSocket/PTTY + fixed `/opt/bin/sh -il`.

Keenetic NDM Console — отдельная вкладка с fixed server-side `ndmc`. Browser выбирает только mode enum `entware|keenetic`, не executable/argv.

Аппаратный PASS на Keenetic Ultra KN-1812:
- exact Admin package install;
- `ndmc show version`;
- browser Entware PTY;
- browser Keenetic PTY;
- repeated Entware ↔ Keenetic switching.

## Monitoring 4→1

Старые split packages:

```text
routerforge-system
routerforge-thermal
routerforge-storage
routerforge-network
```

заменены `routerforge-monitoring`.

Один runtime обслуживает System/Thermal/Storage/Network, а compatibility sockets/API позволяют мигрировать старые установки без второго LAN listener.

Package metadata использует `Provides/Conflicts/Replaces`; postinst останавливает old services и очищает stale sockets. ARM64 migration проверена на реальном Keenetic hardware, включая reboot/autostart.

## DNS

DNS остаётся отдельным Module ABI v1 runtime.

### Resolver safety
`snapshot → validate → mutation → save → readback → semantic compare → verified rollback`.

### Observability
- plain DNS / DoT / DoH;
- clients/LAN/Wi-Fi;
- fallback/timeout/error/latency;
- quality windows/error bursts;
- policy-routing-aware diagnostics.

### Hardening
- kernel BPF filtering before userspace;
- bounded TTL/last-good caches;
- failure backoff;
- bounded frontend read timeouts;
- compact internal event representation with public API unchanged and logical retention kept at **10,000** events.

Это структурная memory optimization; релиз не заявляет непроверенную «магическую» экономию RSS.

## App Center

Единая поверхность для RouterForge, Integrations, Entware, Installed и Updates.

Lifecycle:
- preflight;
- dependencies/sizes;
- guarded async jobs;
- global package-manager lock;
- SSE output;
- timeout/cancel;
- post-action refresh;
- installed-version verification;
- bounded history.

Core self-update остаётся restart-aware и идёт последним в bulk update.

## Generic Runtime Web UI Discovery

Pipeline: `LISTEN socket → PID/process → package → bounded local web probe`.

Без blind LAN/subnet scan. Redirect, XFO, CSP и SSRF boundaries fail closed; infrastructure/platform servers фильтруются, known integrations дедуплицируются.

## Runtime / frontend hardening

- serial polling вместо overlapping async `setInterval`;
- AbortController timeouts для reads;
- stale-while-revalidate/singleflight для thermal;
- bounded ndmc metadata caches;
- bounded App Center finished jobs;
- failed-login client cap 1024 + stale pruning;
- HTTP read/header/idle/header-size limits;
- no global WriteTimeout, чтобы не ломать SSE;
- module mutation body limits before Unix-socket forwarding.

## Release / supply chain

Stable 0.7.1 продвигается только из exact successful Dev FULL RELEASE artifact и того же SHA в `main`.

Публикуются:
- rolling `routerforge-stable`;
- immutable `routerforge-v0.7.1`;
- 5 packages × 3 targets = 15 IPK;
- 3 target indexes;
- SHA256SUMS;
- universal + target bootstraps.

Beta train имеет fail-closed consistency guard между `release_version`, component versions и `min_core_version`.

## CPU architectures

- ARM64 / `aarch64-3.10`: primary production target, fully hardware validated.
- MIPSel / `mipsel-3.4`: experimental, partial physical evidence on Keenetic Giga KN-1010.
- MIPS / `mips-3.4`: experimental, no physical hardware validation.

## Upgrade from 0.6.x

1. Запустить Stable bootstrap.
2. Открыть App Center.
3. Нажать **Проверить обновления**.
4. Обновить installed RouterForge capabilities.
5. Если были split monitoring packages — проверить migration.

Fresh install остаётся Core-only; optional capabilities выбираются после первого запуска.

---

## English summary

RouterForge 0.7.1 brings the 0.7 development train to Stable: consolidated Monitoring, production Management v2 UI, File Manager, Entware + Keenetic terminals, hardened DNS runtime, richer App Center, safe local Web UI discovery, multiarch artifacts and exact-SHA Stable promotion.

ARM64 is the primary supported target. MIPSel remains experimental with partial physical validation, and MIPS big-endian remains experimental without physical hardware validation.
