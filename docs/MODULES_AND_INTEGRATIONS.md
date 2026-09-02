# Modules, Integrations and Marketplace

DNS Monitor развивается как лёгкое ядро с подключаемыми модулями и отдельным слоем интеграций.

## Principles

1. **DNS Core stays small.** Базовый пакет должен оставаться полезным без дополнительных компонентов.
2. **Modules are optional.** System, storage, network, admin and profiling должны устанавливаться/включаться только по желанию.
3. **Integrations are not bundled third-party software.** AWG Manager, nfqws/nfqws2, HydraRoute Neo и другие проекты остаются внешними проектами.
4. **Official sources only by default.** Marketplace хранит метаданные и install plan, но получает сторонние пакеты из официальных источников проекта.
5. **Detect before manage.** Уже установленная вручную система помечается как `installed_external`; DNS Monitor не присваивает её себе.
6. **Read-only first.** Первая реализация Marketplace не выполняет install/update/remove/service control.
7. **Privileged actions require authentication.** Файлы, terminal, service control и package management не должны появляться до отдельной модели авторизации и разрешений.
8. **No arbitrary shell from manifests.** Каталог не должен превращать внешний JSON в root-shell. Исполняемые операции позже будут реализованы через ограниченный набор типизированных действий.

## Layers

```text
DNS Monitor Core
├── DNS observability
├── Web UI / API
├── Module registry
├── Integration registry
└── Marketplace catalog
    ├── compatibility
    ├── repository metadata
    ├── install-plan preview
    └── detection

Optional DNS Monitor modules
├── system
├── thermal
├── storage
├── network
├── admin
└── profiling

Third-party integrations
├── AWG Manager
├── nfqws
├── nfqws2
├── HydraRoute Neo
└── future adapters
```

## Catalog states

- `installed` — встроенный/управляемый компонент DNS Monitor.
- `installed_external` — сторонняя система обнаружена на роутере, но не устанавливалась Marketplace.
- `available` — известная интеграция, которая не обнаружена.
- `planned` — модуль DNS Monitor заложен в roadmap, но физического пакета ещё нет.
- `incompatible` — требования текущего устройства не выполнены.

## Capability model

Read-only capabilities:

```text
detect
version
service-status
open-ui
compatibility
install-preview
```

Future privileged capabilities:

```text
repository-add
repository-remove
install
upgrade
remove
service-start
service-stop
service-restart
```

Привилегированные capabilities должны быть недоступны без auth/permissions layer.

## Packaging target

Future package layout:

```text
dns-monitor
dns-monitor-system
dns-monitor-thermal
dns-monitor-storage
dns-monitor-network
dns-monitor-marketplace
dns-monitor-admin
dns-monitor-profiling
```

Third-party software is **not** copied into these packages. Integration adapters describe detection, compatibility and official install sources.

## Phase plan

### Phase 1 — Catalog foundation

- registry types;
- read-only `/api/catalog`;
- UI storefront;
- detection of AWG Manager, nfqws, nfqws2 and HydraRoute Neo;
- install-plan preview;
- tests.

### Phase 2 — Router inventory and compatibility

- Keenetic model/firmware;
- installed KeeneticOS components;
- Entware architecture and storage;
- repository health;
- conflict/dependency resolver.

### Phase 3 — Repository Manager

- list Entware feeds;
- validate reachability;
- add/remove approved feeds;
- show which catalog item owns/recommends a feed.

### Phase 4 — Safe installation

- explicit confirmation screen;
- typed install operations;
- transaction log;
- rollback where possible;
- package conflict warnings.

### Phase 5 — Administration

Only after authentication/authorization:

- service control;
- updates/removal;
- files;
- opkg UI;
- terminal.
