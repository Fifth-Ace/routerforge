# Официальные модули RouterForge

RouterForge состоит из Core и independently versioned capabilities.

## Пакеты

| Package | UI | Runtime | Назначение |
| --- | --- | --- | --- |
| `routerforge-core` | всегда | Core process | UI, API, SSE, auth, Marketplace, release logic, DNS engine |
| `routerforge-dns` | DNS | capability marker | включает DNS-раздел и DNS runtime Core |
| `routerforge-admin` | Управление | Unix-socket helper | процессы, порты, services, packages, system summary |
| `routerforge-system` | Мониторинг | Unix-socket helper | CPU, RAM/swap, uptime/load, process count |
| `routerforge-thermal` | Мониторинг | Unix-socket helper | thermal/hwmon и доступные датчики |
| `routerforge-storage` | Мониторинг | Unix-socket helper | mounts, capacity, block devices, passive I/O |
| `routerforge-network` | Мониторинг | Unix-socket helper | interfaces, routes, RX/TX, drops/errors, wireless, conntrack |
| `routerforge-profiling` | служебный | Core feature | loopback-only pprof/slow-request profiling |

## Core

`routerforge-core` обязателен.

Он слушает TCP `:2233` и содержит:

- embedded SvelteKit frontend;
- REST API и SSE;
- authentication layer;
- Marketplace catalog;
- remote Registry;
- release-index updater;
- package lifecycle executor;
- DNS engine.

Core не должен зависеть от optional monitoring helpers.

## DNS

`routerforge-dns` — отдельный lifecycle package.

В текущей архитектуре DNS engine компилируется в Core, а пакет включает capability marker:

```text
/opt/etc/routerforge/dns.enabled
```

Это позволяет устанавливать/удалять DNS как пользовательскую capability без второго packet-capture процесса.

## RouterForge Control

Package:

```text
routerforge-admin
```

Control helper остаётся **read-only**. Он предоставляет Core данные о:

- процессах;
- listening sockets;
- Entware init scripts;
- установленных packages;
- system summary;
- storage/thermal inventory, используемый соответствующими UI-сценариями.

Helper работает через root-owned Unix socket и не слушает LAN TCP port.

Package management в Marketplace выполняет **Core по типизированным lifecycle-планам**, а не Control helper.

## Monitoring helpers

`system`, `thermal`, `storage`, `network` запускаются отдельно и общаются с Core через Unix sockets.

Это даёт две важные границы:

1. ненужный модуль можно полностью удалить;
2. helper не обязан открывать собственный Web UI/TCP listener.

Проверка сокетов:

```sh
ls -l /opt/var/run/routerforge-*.sock 2>/dev/null
```

## Profiling

`routerforge-profiling` включает profiling в Core.

Default:

```text
127.0.0.1:6061
```

Core не разрешает non-loopback profiling listener. Для удалённого доступа используйте SSH port forwarding.

## Независимые версии

Каждый компонент имеет собственный version entry в channel config и release-index.

Пример корректного состояния:

```text
routerforge-core     0.3.1
routerforge-dns      0.3.0
routerforge-network  0.3.1
```

Это не рассинхронизация: версии намеренно независимы.

Совместимость модуля с Core задаётся metadata `min_core_version`.

## Установка и удаление

Рекомендуемый lifecycle — через Marketplace.

Удаление optional capability не должно удалять Core или остальные RouterForge-модули.
