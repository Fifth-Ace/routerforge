# Архитектура RouterForge

## Runtime

```text
                            :2233
Browser ──────────────────────┐
                             ▼
                     RouterForge Core
                     /opt/bin/routerforge
                     ├── UI / REST / SSE
                     ├── auth
                     ├── Marketplace
                     ├── release updater
                     ├── DNS engine
                     └── Unix socket clients
                          │
          ┌───────────────┼───────────────┬───────────────┐
          ▼               ▼               ▼               ▼
       Control          System          Thermal         Storage ...
```

Core — единственный RouterForge process, который должен слушать web TCP port.

## Processes and sockets

Official helpers communicate over root-owned Unix sockets under:

```text
/opt/var/run/
```

Например monitoring providers используют `routerforge-*.sock`.

Control source binary всё ещё находится в историческом internal path `cmd/dns-monitor-admin`, но public package/runtime identity — RouterForge.

## Frontend

Frontend:

- Svelte 5 / SvelteKit / Vite;
- static adapter;
- production build встраивается в Go binary;
- Node.js на роутере не нужен.

Основная пользовательская IA:

```text
Главная
Мониторинг      # если установлен хотя бы один telemetry module
DNS             # если установлен routerforge-dns
Управление      # если установлен routerforge-admin
Marketplace
Настройки
```

Навигация capability-driven: отсутствующий optional package не должен оставлять мёртвый раздел.

## Data model

Core агрегирует:

- Keenetic discovery через `ndmc`;
- passive DNS capture;
- локальное состояние opkg;
- module provider data;
- Marketplace Registry;
- current release-index;
- auth/session state.

## DNS boundary

DNS capture/diagnostics остаются внутри Core.

`routerforge-dns` управляет включением capability, а не запускает второй DNS daemon.

## Admin boundary

RouterForge Control helper остаётся read-only.

Root package mutations Marketplace выполняются в Core через ограниченные lifecycle methods и только для разрешённых catalog actions.

## Authentication

Config:

```text
/opt/etc/routerforge/security.json
```

При `auth_required=true` middleware защищает `/api/*`, кроме auth endpoints и health.

Login использует Entware `root`; sessions находятся только в памяти Core.

## Release architecture

Каждый component имеет независимую version.

Source manifests:

```text
packaging/channels/beta.json
packaging/channels/stable.json
```

CI:

```text
dev  ──> RouterForge Beta   ──> routerforge-beta-index.json
main ──> RouterForge Stable ──> routerforge-stable-index.json
```

При неизменной component version предыдущий release asset сохраняется.

Core компилируется с release channel:

- Beta Core читает beta release-index и Registry из `dev`;
- Stable Core читает stable release-index и Registry из `main`.

## Caches

Remote state кэшируется под:

```text
/opt/var/cache/routerforge/
```

Remote refresh throttled примерно до одного раза в час. Manual Marketplace refresh может обходить этот interval.

## Storage policy

DNS runtime history ориентирована на RAM.

RouterForge не должен превращать высокочастотную телеметрию в постоянную запись на flash/USB без отдельной явной функции.
