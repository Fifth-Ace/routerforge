<p align="center">
  <img src="docs/assets/routerforge-banner.jpg" alt="RouterForge — мониторинг, диагностика и управление Keenetic / Netcraze с Entware" width="100%">
</p>

# RouterForge

**Русский** | [English](README_EN.md)

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.7.1-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.7.1--beta.4-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Keenetic%20%2F%20Netcraze-ARM64-blue)](#аппаратная-проверка)

**RouterForge 0.7.1** — модульная веб-платформа для мониторинга, DNS-диагностики, управления и обслуживания роутеров **Keenetic / Netcraze с Entware**.

RouterForge не пытается быть универсальной панелью «для любого Linux». Платформа использует реальные возможности KeeneticOS/NDMS: `ndmc`, RCI, системные данные роутера, DNS, policy routing, таблицы маршрутизации и службы Entware. Core предоставляет единый Web UI, авторизацию, Центр приложений, общий API и хост Module ABI. DNS, Management и Monitoring устанавливаются отдельными пакетами и общаются с Core через root-owned Unix sockets.

Единственный пользовательский TCP-порт RouterForge — **`:2233`**. Служебные модули не размножают внешние HTTP-порты.

> [!NOTE]
> **RouterForge — независимый некоммерческий проект сообщества.** Он не является официальным продуктом, подразделением или партнёрским проектом **Keenetic, Netcraze, Entware** и других упомянутых компаний или проектов. Все товарные знаки принадлежат их владельцам. Лицензия проекта — MIT.

> [!TIP]
> **Core и модули RouterForge могут версионироваться независимо.** Номер платформенного релиза не обязан всегда совпадать с версиями каждого пакета. Для Stable 0.7.1 пять официальных пакетов намеренно сведены в единый релизный train `0.7.1`, но архитектура сохраняет возможность отдельных component releases.

> [!IMPORTANT]
> Основная производственная архитектура — **ARM64 / `aarch64-3.10`**.
>
> - **Keenetic Hopper KN-3811** — основная аппаратная площадка для **Dev и Beta**.
> - **Keenetic Ultra KN-1812** — аппаратная площадка для **Beta и Stable-релиза**.
> - **Keenetic Giga KN-1010 / MIPSel** — частичная физическая проверка, статус experimental.
> - **MIPS big-endian** — experimental без физической аппаратной проверки.
>
> Таким образом, Beta проходит реальные проверки на двух ARM64-площадках: KN-3811 и KN-1812. Подробная матрица — в [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md).

## Сильные стороны RouterForge

### Нативная работа с Keenetic
RouterForge понимает не только Entware/Linux, но и сам роутер: `ndmc`, RCI, DNS, policy routing, системные интерфейсы, маршруты и особенности запуска служб Keenetic.

### Модульность без зоопарка внешних сервисов
Свежая установка начинается с Core. DNS, Management, Monitoring и Profiling подключаются по необходимости. Пользователь работает с одним Web UI, а внутренние runtime-модули используют root-owned Unix sockets.

### Безопасные изменения вместо произвольного shell из браузера
Критичные операции выполняются через ограниченные server-side контракты: фиксированные сигналы и действия служб, live root-session, защита путей, DNS snapshot/readback/rollback и фиксированный `ndmc` для Keenetic NDM Console.

### Расчёт на ресурсы роутера
Используются ограниченные кэши и история, последовательные опросы, backoff/timeouts, компактные DNS event rings и production-compression. Node.js нужен только для сборки фронтенда и **не требуется на роутере**.

### Центр приложений вместо ручного OPKG-зоопарка
Официальные пакеты RouterForge, Integrations, Entware/OPKG, installed state и updates собраны в одном интерфейсе.

### Проверяемая цепочка релиза
Stable строится из заранее проверенного SHA. Release-index фиксирует версии, архитектуру, URL и SHA256. Публикация идёт через отдельный promotion artifact с повторной проверкой.

## Что умеет RouterForge

### Management v2
`routerforge-admin` объединяет процессы, Entware Services, File Manager, Maintenance, Entware Terminal и Keenetic NDM Console.

Процессы поддерживают защищённые `TERM/HUP/INT/KILL`. Entware-службы — `start/stop/restart`.

File Manager:
- **Commander / Explorer**;
- дерево каталогов и тома;
- UTF-8 редактор;
- создание каталогов;
- rename/move;
- download/delete;
- properties;
- visual `chmod`;
- guarded boundary для `/opt` и `/tmp`.

**Entware Terminal** — WebSocket/PTTY + фиксированный `/opt/bin/sh -il`.

**Keenetic NDM Console** — WebSocket/PTTY + фиксированный server-side `ndmc`. Browser выбирает только `entware` или `keenetic`, а не executable/argv.

### Monitoring
Вместо `routerforge-system`, `routerforge-thermal`, `routerforge-storage`, `routerforge-network` используется единый `routerforge-monitoring`, обслуживающий System / Thermal / Storage / Network.

### DNS
`routerforge-dns` — отдельный Module ABI v1 runtime с:
- plain DNS / DoT / DoH;
- Add/Edit/Delete/Disable/Enable;
- read-only защитой динамических DNS;
- logical multi-domain resolvers;
- `snapshot → validation → mutation → save → readback → verified rollback`;
- client/LAN/Wi-Fi attribution;
- fallback/timeout/error/latency diagnostics;
- route-aware diagnostics;
- компактными event rings при логической глубине истории **10 000** событий.

### Центр приложений
RouterForge / Integrations / Entware / Installed / Updates, package preflight, dependencies/sizes, guarded async jobs, global package-manager lock, timeout/cancel, post-action verification, bulk update и безопасное обнаружение локальных Web UI без blind LAN scan.

### Настройки и авторизация
При включённой авторизации используется Entware-пользователь `root`; пароль RouterForge не сохраняет; session token хранится в RAM; cookie использует `HttpOnly` + `SameSite=Strict`; failed-login tracking ограничен и rate-limited. Конфигурация: `/opt/etc/routerforge/security.json`.

## Архитектура

```text
Browser
   |
   | http://router:2233
   v
RouterForge Core
├── Web UI / REST / SSE
├── Authentication
├── Центр приложений + Registry
├── Release index / package lifecycle
├── Generic Module API + UI host
└── Unix-socket proxy
     ├── routerforge-dns
     ├── routerforge-admin
     │    ├── Processes / Services
     │    ├── File Manager / Maintenance
     │    ├── Entware Terminal
     │    └── Keenetic NDM Console
     └── routerforge-monitoring
          ├── System
          ├── Thermal
          ├── Storage
          └── Network
```

Внешний Web-порт платформы один: **2233**. `routerforge-profiling` по умолчанию доступен только на `127.0.0.1:6061`.

## Официальные пакеты Stable 0.7.1

| Пакет | Назначение |
| --- | --- |
| `routerforge-core` | Web UI, авторизация, Центр приложений, Registry/release lifecycle, хост Module ABI |
| `routerforge-dns` | DNS runtime, UI, управление резолверами и диагностика |
| `routerforge-admin` | Management v2, File Manager, Maintenance, Entware Terminal и Keenetic NDM Console |
| `routerforge-monitoring` | единый System/Thermal/Storage/Network runtime + UI |
| `routerforge-profiling` | локальное профилирование Core |

Свежая установка ставит Core. Остальные возможности выбираются через Центр приложений.

## Аппаратная проверка

| Устройство | Архитектура | Каналы | Роль |
| --- | --- | --- | --- |
| **Keenetic Hopper KN-3811** | ARM64 | **Dev + Beta** | ежедневная разработка, функциональные проверки и Beta |
| **Keenetic Ultra KN-1812** | ARM64 | **Beta + Stable** | финальная Beta-проверка и проверка релизной версии |
| **Keenetic Giga KN-1010** | MIPSel | experimental | частичная физическая проверка установки и базовой работы |

CI дополнительно проверяет cross-build, QEMU для MIPS/MIPSel, структуру IPK, release indexes, bootstrap scripts и целостность артефактов. CI не подменяет физическую проверку на роутере.

## Требования

- Keenetic или Netcraze с KeeneticOS/NDMS;
- Entware в `/opt`;
- рабочий `opkg`;
- root-доступ через Entware;
- `sha256sum`;
- `curl` или `wget`;
- production-рекомендация: ARM64 / `aarch64-3.10`.

## Полный патчноут 0.7.1

Полный список изменений **относительно Stable 0.6.1**:
**[RouterForge 0.7.1 — полный патчноут](docs/RELEASE_NOTES_0.7.1.md)**

## Быстрая установка — Stable

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

После установки: `http://<ip-роутера>:2233`

## Обновление с Stable 0.6.1

1. Запустить Stable bootstrap.
2. Открыть **Центр приложений**.
3. Нажать **«Проверить обновления»**.
4. Обновить установленные RouterForge packages.
5. Проверить migration split monitoring → `routerforge-monitoring`.

## Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Stable и Beta — разные rolling channels. Не смешивайте их без осознанной смены канала.

## Сервис и диагностика

```sh
/opt/etc/init.d/S90routerforge restart
tail -f /opt/var/log/routerforge.log
wget -qO- http://127.0.0.1:2233/api/health
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
ls -l /opt/var/run/routerforge-*.sock 2>/dev/null
```

## Удаление

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```

## Документация

- [Полный патчноут RouterForge 0.7.1](docs/RELEASE_NOTES_0.7.1.md)
- [Установка и обновление](docs/INSTALLATION.md)
- [Модули](docs/MODULES.md)
- [Management v2](docs/MANAGEMENT_V2_API.md)
- [File Manager API](docs/MANAGEMENT_V2_FILES_API.md)
- [Центр приложений](docs/MARKETPLACE.md)
- [Архитектура](docs/ARCHITECTURE.md)
- [CPU-архитектуры и аппаратные проверки](docs/ARCHITECTURES.md)
- [Миграция Monitoring](docs/MONITORING_MIGRATION.md)
- [Процесс релиза](docs/RELEASE_PROCESS.md)
- [Диагностика](docs/TROUBLESHOOTING.md)
- [Changelog](CHANGELOG.md)
- [Политика безопасности](SECURITY.md)
- [Участие в разработке](CONTRIBUTING.md)

## Сборка из исходников

Backend: Go 1.21+. Frontend: Node.js 22.x.

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
gofmt -w .
go test ./...
go vet ./...
```

Node.js на роутере не требуется.

## Лицензия

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
