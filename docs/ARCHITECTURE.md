# Архитектура RouterForge

## Runtime

```text
Browser
   │
   │ http://router:2233
   ▼
RouterForge Core
/opt/bin/routerforge
├── Web shell / REST / SSE
├── Authentication
├── Marketplace + Registry
├── Release index / package lifecycle
├── Generic Module API + UI host
└── Unix-socket proxy
     ├── RouterForge DNS
     │   /opt/bin/routerforge-dns
     │   /opt/var/run/routerforge-dns.sock
     │   ├── discovery / capture / health
     │   ├── resolver observability
     │   ├── DNS Control + readback / rollback
     │   └── independent module UI
     ├── RouterForge Control
     │   /opt/bin/routerforge-admin
     ├── System Monitor
     ├── Thermal Monitor
     ├── Storage Monitor
     └── Network Monitor
```

Core — единственный RouterForge process, который слушает пользовательский TCP-порт **2233**.
Официальные runtime-модули и Control общаются с Core через root-owned Unix sockets и не открывают отдельные LAN TCP-порты.

## Source layout

Репозиторий организован по владельцу компонента:

```text
components/core/             Core backend + frontend + packaging
components/control/          RouterForge Control + packaging
modules/dns/                 DNS runtime + frontend + packaging
modules/monitoring-runtime/  shared System/Thermal/Storage/Network runtime
modules/<id>/packaging/      package-specific lifecycle/init files
release/channels/            beta/stable source manifests
```

Core и DNS по-прежнему собираются из explicit source lists в `scripts/build-opkg.sh` и `scripts/build-module-opkg.sh`. Физическое разделение директорий дополнительно закрепляет границу Module ABI v1: изменение DNS runtime не должно молча менять Core binary.

System/Thermal/Storage/Network намеренно используют общий `modules/monitoring-runtime/`: это один read-only Unix-socket runtime с выбором `-module`, а package/init ownership остаётся у соответствующих `modules/<id>/`.

`marketplace/registry/index.json` сохраняет исторический публичный GitHub path для совместимости со старыми Core; точная embedded-копия Core проверяется генератором Registry и CI.

Исторические материалы, не участвующие в build/runtime, складываются в `archive/`. Подробная карта: [REPOSITORY_LAYOUT.md](REPOSITORY_LAYOUT.md).

## Module ABI v1

Package installation и runtime readiness — разные состояния.

Core определяет:

1. установлен ли package по локальной базе `opkg`;
2. доступен ли runtime через его Unix socket;
3. отвечает ли module health endpoint.

Во время install/update/restart package может уже быть установлен, пока socket ещё не поднялся.
В этот промежуток Core не должен объявлять модуль отсутствующим:

- module API возвращает структурированный `503` с `installed=true`, `running=false`;
- UI делает health preflight/retry;
- iframe proxy показывает reconnect page вместо сырого JSON;
- после восстановления health модуль возвращается без ручного refresh.

Loopback `GET/HEAD /api/modules/<id>/health` используется maintainer scripts как readiness probe.

## DNS boundary

`routerforge-dns` — самостоятельный Module ABI v1 runtime.

В DNS package находятся:

- Keenetic DNS discovery;
- passive DNS/client capture;
- resolver/routing diagnostics;
- DNS health/quality/history;
- DNS mutation API;
- snapshot → mutation → save → readback;
- verified rollback;
- DNS module UI.

Core не реализует DNS mutation/capture логику. Он предоставляет общий web shell, auth, Marketplace и generic module proxy.

## Control boundary

`routerforge-admin` (RouterForge Control) остаётся отдельным read-only helper.

Root package mutations выполняет Core только через ограниченный Marketplace lifecycle и только для разрешённых catalog actions.

## Marketplace and package state

Каждый component имеет независимую version.

Source channel manifests:

```text
release/channels/beta.json
release/channels/stable.json
```

CI:

```text
dev  ──> RouterForge Beta   ──> routerforge-beta-index.json
main ──> RouterForge Stable ──> routerforge-stable-index.json
```

При неизменной component version предыдущий release asset сохраняется.

Installed version читается из `opkg`. Core учитывает только package stanza, чей state действительно `installed`;
старые `not-installed` tombstones не должны перекрывать текущую версию.

После `update` lifecycle дополнительно сверяет фактически установленную version с target release version.

## Authentication

Config:

```text
/opt/etc/routerforge/security.json
```

При `auth_required=true` middleware защищает `/api/*`, кроме публичных auth endpoints, Core health и loopback-only module health readiness probe.

Login использует Entware `root`; sessions находятся только в памяти Core.

## Frontend

Frontend:

- Svelte 5 / SvelteKit / Vite;
- static adapter;
- production build встраивается в Core Go binary;
- Node.js на роутере не нужен;
- capability-driven navigation скрывает разделы отсутствующих optional packages.

Основная IA:

```text
Главная
Мониторинг
DNS
Управление
Marketplace
Настройки
```

## CPU / Entware architectures

Публичный release target сейчас:

```text
aarch64-3.10  -> GOARCH=arm64
```

Build system заранее содержит compile/package profiles:

```text
mips-3.4      -> GOARCH=mips   + GOMIPS=softfloat
mipsel-3.4    -> GOARCH=mipsle + GOMIPS=softfloat
```

MIPS/MIPSEL пока являются **CI-only planned targets** и не публикуются как поддерживаемые RouterForge releases.
Подробнее: [ARCHITECTURES.md](ARCHITECTURES.md).

## Caches and storage policy

Remote Marketplace state кэшируется под:

```text
/opt/var/cache/routerforge/
```

Remote refresh throttled примерно до одного раза в час; manual Marketplace refresh может обходить interval.

Высокочастотная DNS/runtime телеметрия ориентирована на RAM.
RouterForge не должен превращать её в постоянную запись на flash/USB без отдельной явной функции.
