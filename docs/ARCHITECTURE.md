# Архитектура RouterForge

## Runtime Stable 0.7.1

```text
Browser -> RouterForge Core :2233
           ├── Authentication / App Center / lifecycle
           ├── Generic Module API/UI host
           └── root-owned Unix-socket proxy
                ├── routerforge-dns
                ├── routerforge-admin
                └── routerforge-monitoring
```

Core — единственный RouterForge process с пользовательским LAN listener `:2233`.
Profiling, если установлен, слушает только loopback `127.0.0.1:6061`.

## Module ABI
Package-installed и runtime-ready — разные состояния. Core читает `opkg`, manifest/socket и health; во время restart UI использует reconnect semantics.

## DNS boundary
`routerforge-dns` владеет capture/discovery/health/history/resolver mutations/UI. Core не интерпретирует DNS payload и не реализует DNS writes.

## Management boundary
`routerforge-admin` владеет Processes, Services, File Manager, Maintenance и PTY backend.

Mutations требуют live Entware-root session, same-origin, confirmation/whitelist и Core-injected internal marker.

Terminal modes фиксированы:
- `entware` → `/opt/bin/sh -il`;
- `keenetic` → server-resolved `ndmc`.

Request-controlled executable отсутствует.

## Monitoring
Stable 0.7.1 публикует consolidated `routerforge-monitoring`: один read-only runtime + UI и intentional compatibility sockets System/Thermal/Storage/Network.

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

## Generic Web UI Discovery
Локальные application listeners коррелируются с PID/process/package и bounded web probe. Blind LAN scan отсутствует; redirect/XFO/CSP/SSRF checks fail closed.

## Source layout
См. [REPOSITORY_LAYOUT.md](REPOSITORY_LAYOUT.md).

## Frontend
Svelte 5/SvelteKit/Vite. Core UI embedded; DNS/Admin/Monitoring UIs входят в свои IPK. Node.js на роутере не нужен.
