# Официальные модули RouterForge

RouterForge состоит из Core и independently versioned capabilities.

## Пакеты

| Package | UI | Runtime | Назначение |
| --- | --- | --- | --- |
| `routerforge-core` | всегда | Core process | Web shell, auth, Marketplace, Registry, lifecycle, Module ABI host |
| `routerforge-dns` | DNS | Unix-socket runtime | DNS observability, resolver management, traffic, diagnostics |
| `routerforge-admin` | Управление | Unix-socket helper | процессы, порты, services, packages, system summary |
| `routerforge-system` | Мониторинг | Unix-socket helper | CPU, RAM/swap, uptime/load, process count |
| `routerforge-thermal` | Мониторинг | Unix-socket helper | thermal/hwmon и доступные датчики |
| `routerforge-storage` | Мониторинг | Unix-socket helper | mounts, capacity, block devices, passive I/O |
| `routerforge-network` | Мониторинг | Unix-socket helper | interfaces, routes, RX/TX, drops/errors, wireless, conntrack |
| `routerforge-profiling` | служебный | Core feature | loopback-only pprof/slow-request profiling |

## Core

`routerforge-core` обязателен и слушает TCP `:2233`.

Начиная с Module ABI v1 Core содержит только платформенные функции:

- embedded SvelteKit shell;
- authentication layer;
- Marketplace catalog and package lifecycle;
- remote Registry and release-index updater;
- generic `/api/modules/<id>/...` Unix-socket proxy;
- generic module UI host/frame;
- compatibility bridges needed during migrations.

DNS packet capture, resolver discovery, DNS health checks and DNS configuration writes are not part of the production Core binary in the 0.4 generation.

ABI marker:

```text
/opt/etc/routerforge/module-abi-v1
```

It is owned by `routerforge-core` and lets optional runtimes know that the generic module host is available.

## DNS

Package:

```text
routerforge-dns
```

DNS is a full independent module. It installs:

```text
/opt/bin/routerforge-dns
/opt/etc/init.d/S91routerforge-dns
/opt/var/run/routerforge-dns.sock
/opt/share/routerforge/modules/dns/manifest.json
/opt/share/routerforge/modules/dns/ui/
```

The module owns:

- DNS packet/client capture;
- Keenetic resolver discovery and health;
- plain DNS / DoT / DoH observability;
- resolver Add/Edit/Delete;
- temporary Disable/Enable metadata;
- native multi-domain grouping;
- structured Keenetic RCI mutations and readback/rollback;
- DNS-specific UI.

Core proxies the module through:

```text
/api/modules/dns/...
```

The compatibility marker remains during the 0.3 → 0.4 migration:

```text
/opt/etc/routerforge/dns.enabled
```

With an old 0.3 Core it still enables the legacy in-Core DNS engine. With Core 0.4+, `S91routerforge-dns` is the authoritative DNS runtime. This prevents two packet-capture engines from running at the same time during an ordered package upgrade.

## Module API boundary

Monitoring modules remain read-only. DNS is the first Module ABI v1 runtime with a mutation API.

Core does not interpret DNS payloads. It authenticates the user, enforces the platform route, and forwards allowed methods to the root-owned Unix socket. DNS itself validates resolver input and performs router mutations.

Current DNS API surface:

```text
GET    /api/modules/dns/health
GET    /api/modules/dns/snapshot
GET    /api/modules/dns/history
GET    /api/modules/dns/quality
GET    /api/modules/dns/fallbacks
GET    /api/modules/dns/error-bursts
GET    /api/modules/dns/clients
GET    /api/modules/dns/client
GET    /api/modules/dns/interfaces
GET    /api/modules/dns/system
GET    /api/modules/dns/plain-dns
GET    /api/modules/dns/info
GET    /api/modules/dns/resolvers
POST   /api/modules/dns/resolvers
PATCH  /api/modules/dns/resolvers/<id>
DELETE /api/modules/dns/resolvers/<id>
POST   /api/modules/dns/resolvers/<id>/disable
POST   /api/modules/dns/resolvers/<id>/enable
POST   /api/modules/dns/preview
```

The normal DNS UI consumes the observability endpoints lazily by active view. Resolver writes remain confined to the mutation endpoints above and keep the same snapshot/readback/rollback safety boundary.

Mutation requests require the RouterForge action header injected by the official module UI. The DNS runtime also rejects unknown JSON fields and limits request body size.

## Module lifecycle и restart

Наличие package и готовность runtime — разные состояния. Core сначала определяет установку по `opkg`, затем проверяет module health через Unix-socket proxy.

При install/update/restart:

1. package может быть уже установлен, пока socket ещё не поднялся;
2. обычный module API в этот момент возвращает структурированный `503`;
3. UI frame использует health preflight и показывает reconnect state;
4. iframe proxy на временной dial-ошибке отдаёт no-store reconnect page вместо сырого JSON;
5. после появления health UI автоматически возвращается к модулю.

Это позволяет обновлять Module ABI runtime без ручного refresh страницы и без ложного состояния «модуль не установлен».

## DNS write safety

DNS mutations use the following safety model:

```text
saved RCI snapshot
    ↓
validation / normalization
    ↓
replace only affected protocol section
    ↓
configuration save
    ↓
RCI readback + semantic comparison
    ↓
OK or restore previous section + save + verify rollback
```

Dynamic DNS entries supplied by DHCP or another Keenetic service are displayed but are read-only.

Temporarily disabled entries are stored by RouterForge because Keenetic does not expose a native disabled state for resolver entries.

## RouterForge Control

Package:

```text
routerforge-admin
```

Control helper remains read-only. It provides Core data about processes, listening sockets, Entware init scripts, installed packages and system summary.

Package management in Marketplace is performed by Core through typed lifecycle plans, not by the Control helper.

## Monitoring helpers

`system`, `thermal`, `storage`, `network` run separately and communicate with Core through Unix sockets.

This keeps two important boundaries:

1. an unused module can be removed completely;
2. a helper does not need to open its own LAN TCP listener.

Socket check:

```sh
ls -l /opt/var/run/routerforge-*.sock 2>/dev/null
```

## Profiling

`routerforge-profiling` enables profiling in Core.

Default:

```text
127.0.0.1:6061
```

Core does not permit a non-loopback profiling listener. Use SSH port forwarding for remote access.

## Independent versions

Every component has its own version in the channel release index.

A normal post-ABI state can look like:

```text
routerforge-core     0.4.3
routerforge-dns      0.4.18
routerforge-network  0.3.2
```

This is intentional. DNS-only changes must not require a Core version bump after Module ABI v1 has landed.

Compatibility is expressed through `min_core_version` metadata.

## Installation and removal

The recommended lifecycle is Marketplace.

Removing an optional capability must not remove Core or any other RouterForge module.
