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
├── Центр приложений + Registry
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
     └── RouterForge Monitoring
         /opt/bin/routerforge-monitoring
         ├── System
         ├── Thermal
         ├── Storage
         └── Network
```

Core — единственный RouterForge process, который слушает пользовательский TCP-порт **2233**.
Официальные runtime-модули и Control общаются с Core через root-owned Unix sockets и не открывают отдельные LAN TCP-порты.

## Source layout

Репозиторий организован по владельцу компонента:

```text
components/core/             Core backend + frontend + packaging
components/control/          RouterForge Control backend + packaging
modules/admin/frontend/      standalone RouterForge Control UI
modules/dns/                 DNS runtime + frontend + packaging
modules/monitoring-runtime/  consolidated Monitoring runtime + packaging
modules/monitoring/frontend/ standalone Monitoring UI
modules/<id>/packaging/      legacy split-package compatibility lifecycle
release/channels/            dev/beta/stable source manifests
```

Core и DNS по-прежнему собираются из explicit source lists в `scripts/build-opkg.sh` и `scripts/build-module-opkg.sh`. Физическое разделение директорий дополнительно закрепляет границу Module ABI v1: изменение DNS runtime не должно молча менять Core binary.

Текущий Dev/Beta train публикует consolidated `routerforge-monitoring`: один read-only runtime поднимает основной Monitoring socket и совместимые System/Thermal/Storage/Network sockets. Старые split packages сохраняются только как migration/compatibility boundary и объявлены через `Provides/Conflicts/Replaces`.

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

Core не реализует DNS mutation/capture логику. Он предоставляет общий web shell, auth, Центр приложений и generic module proxy.

## Control boundary

`routerforge-admin` (RouterForge Control) остаётся отдельным Unix-socket helper. Read endpoints покрывают процессы, listening sockets, Entware services/packages и summary.

Management v2 добавляет строго ограниченные mutation endpoints для process signals и Entware service start/stop/restart. Они требуют live Entware-root session, same-origin, exact confirmation, whitelist и Core-injected internal socket marker. Текущий standalone Management UI остаётся read-only.

Root package mutations по-прежнему выполняет Core только через ограниченный Центр приложений lifecycle и только для разрешённых catalog actions.

## Центр приложений и package state

Каждый component имеет независимую version.

Source channel manifests:

```text
release/channels/dev.json
release/channels/beta.json
release/channels/stable.json
```

Release lifecycle:

```text
push dev
  └──> rolling ARM64 Dev (routerforge-dev)

workflow_dispatch FULL RELEASE on exact dev SHA
  └──> rolling Beta + immutable routerforge-v<beta-version>

main
  └──> Stable promotion from validated exact-SHA artifact
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

Failed-login state также in-memory, но bounded: stale entries очищаются глобально, tracked clients ограничены 1024, активные lockout записи сохраняются приоритетно. Lockout policy остаётся 5 ошибок в 5-минутном окне с 30-секундной блокировкой.

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
Центр приложений
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

MIPS/MIPSEL публикуются как **experimental preview** targets. Cross-build, QEMU и runtime compatibility probe являются CI-доказательством совместимости сборки, но не заменяют физическую hardware validation. MIPSel уже имеет частичную physical evidence на KN-1010 (fresh install/basic operation); MIPS big-endian остаётся без физической проверки.
Подробнее: [ARCHITECTURES.md](ARCHITECTURES.md).

## Caches and storage policy

Remote состояние Центра приложений кэшируется под:

```text
/opt/var/cache/routerforge/
```

Remote refresh throttled примерно до одного раза в час; manual Центр приложений refresh может обходить interval.

Высокочастотная DNS/runtime телеметрия ориентирована на RAM.
RouterForge не должен превращать её в постоянную запись на flash/USB без отдельной явной функции.
