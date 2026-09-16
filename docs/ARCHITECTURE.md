# Архитектура RouterForge

## Runtime Stable 0.9.0

```text
Browser -> RouterForge Core :2233
           ├── Authentication / App Center / Registry / lifecycle
           ├── Jobs + History / channel transition control
           ├── Generic Module API/UI host
           └── root-owned Unix-socket proxy
                ├── routerforge-dns
                ├── routerforge-admin
                ├── routerforge-monitoring
                └── routerforge-network-tools
```

Core — единственный RouterForge process с пользовательским LAN listener `:2233`.
Profiling, если установлен, слушает только loopback `127.0.0.1:6061`.

## Module ABI

Package-installed и runtime-ready — разные состояния. Core читает `opkg`,
manifest/socket/runtime readiness; во время restart UI использует reconnect/reload semantics.
Module HTML/UI отдаётся `no-store`, fingerprinted assets — immutable.

## Registry / App Center boundary

Bundled manifests — источник истины для integration metadata.
Local/private sources используют namespaced identity, read-only preview и explicit add,
привязанный к exact preview SHA-256. Trust не даёт произвольный shell/lifecycle authority.

## DNS boundary

`routerforge-dns` владеет capture/discovery/rolling health/history/resolver mutations/UI.
Core не интерпретирует DNS payload и не реализует DNS writes.

## Management boundary

`routerforge-admin` владеет Processes, Services, File Manager, Maintenance и PTY backend.

При выключенной RouterForge auth same-origin guarded actions разрешены.
При включённой auth mutation path требует authenticated root session.
Cross-origin, exact confirmation/whitelist, protected-process/path guards и
Core-injected internal marker сохраняются.

Terminal modes фиксированы:
- `entware` → `/opt/bin/sh -il`;
- `keenetic` → server-resolved `ndmc`.

Request-controlled executable отсутствует.

## Shared Safety

`internal/safety` предоставляет reusable path validation, atomic write/swap/rollback
и bounded command execution. Interactive Terminal/PTTY и несколько явно задокументированных
low-level paths остаются intentional exceptions.

## Monitoring

Stable 0.9.0 публикует consolidated `routerforge-monitoring`: один read-only runtime + UI
и intentional compatibility sockets System/Thermal/Storage/Network.
Thermal discovery учитывает Keenetic/NDMS/sysfs sources.

## Network Tools

`routerforge-network-tools` — единственный first-class модуль active network diagnostics.
Он владеет Network Doctor, Route Inspector, Flow Explorer и Active Probes.
Monitoring сохраняет только read-only network telemetry.

## App Center / release lifecycle

```text
push dev -> rolling Dev

FULL RELEASE exact dev SHA
  -> broad validation
  -> optional Beta
  -> stable promotion artifact

same validated SHA -> main
  -> verified rolling Stable
  -> immutable routerforge-v<stable-version>
```

## Forgejo authority

GitHub — primary. Private Forgejo — hot backup.
Нормальная replication: GitHub → Forgejo.
Forgejo → GitHub не выполняется автоматически; promotion/restore fail closed при divergence.

## Generic Web UI Discovery

Локальные application listeners коррелируются с PID/process/package и bounded web probe.
Blind LAN scan отсутствует; redirect/XFO/CSP/SSRF checks fail closed.

## Source layout

См. [REPOSITORY_LAYOUT.md](REPOSITORY_LAYOUT.md).

## Frontend

Svelte 5/SvelteKit/Vite. Core UI embedded; DNS/Admin/Monitoring/Network Tools UIs входят
в свои IPK. Node.js на роутере не нужен.
