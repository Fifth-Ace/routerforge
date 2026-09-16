<p align="center">
  <img src="docs/assets/routerforge-banner.jpg" alt="RouterForge — мониторинг, диагностика и управление Keenetic / Netcraze с Entware" width="100%">
</p>

# RouterForge

**Русский** | [English](README_EN.md)

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.9.0-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.8.5--beta.2-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge 0.9.0** — модульная веб-платформа, созданная специально для роутеров **Keenetic / Netcraze с Entware**: мониторинг, DNS, управление, диагностика сети и обслуживание в одном интерфейсе.

RouterForge работает напрямую с KeeneticOS/NDMS и Entware: `ndmc`, RCI, интерфейсы, маршруты, DNS, policy routing, процессы и службы. Пользователь работает через единый Web UI на **`:2233`**, а внутренние модули подключаются к Core через Unix sockets.

> [!NOTE]
> **RouterForge — независимый некоммерческий проект сообщества.** Он не является официальным продуктом, подразделением или партнёрским проектом **Keenetic, Netcraze, Entware** и других упомянутых компаний или проектов. Все товарные знаки принадлежат их владельцам. Лицензия проекта — MIT.

> [!TIP]
> Core и модули версионируются независимо — [Versioning Policy](docs/VERSIONING.md).

## Почему RouterForge

- **Сделан именно для роутеров Keenetic / Netcraze.** Это не универсальная Linux-панель, перенесённая на роутер: архитектура, runtime и интерфейс изначально строятся вокруг KeeneticOS/NDMS и Entware.
- **Глубокая интеграция с системой роутера.** RouterForge работает с `ndmc`, RCI, policy routing, маршрутами, интерфейсами, DNS и системными службами, а не ограничивается обычными Linux-метриками.
- **Один интерфейс вместо набора разрозненных панелей.** Core объединяет модули, настройки, Центр приложений, диагностику и обслуживание на одном Web-порту `2233`.
- **Модульная установка.** Core ставится первым, а остальные возможности добавляются только если они нужны.
- **Экономный runtime.** Backend на Go, компактные кэши и история, без Node.js на самом роутере и без лишних внешних HTTP-сервисов.
- **Контролируемые изменения.** Чувствительные действия выполняются через ограниченные server-side контракты, проверки путей, подтверждения и rollback там, где он действительно нужен.
- **Проверяемая цепочка релиза.** Stable публикуется из заранее проверенного exact SHA; multi-arch индексы, SHA256 и immutable release snapshot позволяют проверить, что именно установлено.

## Модули RouterForge — Stable 0.9.0

### Core — единая точка управления
`routerforge-core` · **0.9.0**

Сам RouterForge: единый Web UI, авторизация, настройки, Центр приложений, Registry и подключение остальных модулей.

**Нужен всем:** Core — база платформы. С него начинается установка и через него вы управляете остальными возможностями.

### DNS — больше контроля, чем штатные настройки DNS
`routerforge-dns` · **0.8.1**

Не просто список DNS-серверов. Модуль даёт отдельное рабочее место для DNS: обычный DNS, DoT и DoH, управление резолверами, текущий health, задержки, ошибки, fallback и понятное объяснение, **что именно сейчас работает плохо и почему**.

В отличие от штатного управления DNS в прошивке, RouterForge позволяет **временно отключить резолвер без его удаления**, а потом включить обратно. Это удобно для тестов, сравнения провайдеров, поиска проблем и временного исключения нестабильного сервера без потери настроек. Динамические системные резолверы при этом защищены от случайного редактирования.

Дополнительно модуль:
- показывает состояние **каждого резолвера отдельно**, а не только общий итог;
- не держит сервер «жёлтым навсегда» из-за старых ошибок — health считается по текущему rolling window;
- отличает реальные сбои от нормальных `NXDOMAIN`;
- показывает latency/p95, ошибки, fallback и недоступность;
- помогает понять, через какой резолвер реально идут запросы клиентов;
- умеет безопасно применять изменения через `snapshot → validation → mutation → save → readback → rollback`.

**Нужен, если:** хотите не просто «задать DNS», а реально **управлять им, сравнивать резолверы и быстро понимать причину проблем** — особенно при нескольких DNS, DoT/DoH и сложной сетевой политике.

### Management — администрирование без постоянного SSH
`routerforge-admin` · **0.8.1**

Процессы и службы Entware, File Manager, Maintenance, полноценный Entware Terminal и Keenetic NDM Console в одном интерфейсе.

**Нужен, если:** хотите большую часть обычного обслуживания делать из браузера — посмотреть процесс, перезапустить службу, поправить файл, открыть терминал или зайти в `ndmc`, не прыгая между отдельными инструментами.

### Monitoring — всё важное о состоянии роутера
`routerforge-monitoring` · **0.8.0**

System, Thermal, Storage и Network в одном месте: загрузка CPU, память, температура, хранилища и состояние сетевых интерфейсов.

**Нужен, если:** хотите открыть одну страницу и сразу понять, всё ли нормально с роутером — без `top`, `df`, `ip` и ручного поиска датчиков температуры.

### Network Tools — когда нужно понять, куда на самом деле идёт трафик
`routerforge-network-tools` · **0.9.0**

Network Doctor, Route Inspector, Flow Explorer и Active Probes: маршруты, активные потоки, проверки доступности и разбор сетевого пути в одном модуле.

**Нужен, если:** у вас несколько WAN/VPN, policy routing, сложная маршрутизация или периодически возникает вопрос «почему этот адрес идёт не туда?».

### Profiling — диагностика самого RouterForge
`routerforge-profiling` · **0.7.1**

Локальное профилирование Core для разработки и глубокой диагностики производительности.

**Большинству пользователей не нужен:** ставьте только если целенаправленно нужен profiling.

## Архитектура

```text
Browser
   |
   | http://router:2233
   v
RouterForge Core
├── Web UI / REST / SSE
├── Authentication / Settings
├── App Center / Registry / Release lifecycle
├── Module ABI host
└── Unix-socket proxy
     ├── DNS
     ├── Management
     │    ├── Processes / Services
     │    ├── File Manager / Maintenance
     │    ├── Entware Terminal
     │    └── Keenetic NDM Console
     ├── Monitoring
     │    ├── System
     │    ├── Thermal
     │    ├── Storage
     │    └── Network
     └── Network Tools
          ├── Network Doctor
          ├── Route Inspector
          ├── Flow Explorer
          └── Active Probes
```

`routerforge-profiling` работает отдельно и по умолчанию доступен только на `127.0.0.1:6061`.

## Установка

### Stable

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

После установки:

```text
http://<ip-роутера>:2233
```

### Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Stable и Beta — разные rolling channels. Переключайте канал осознанно.

## Поддерживаемая платформа

RouterForge разрабатывается для **Keenetic / Netcraze с Entware**.

| Архитектура | Статус |
| --- | --- |
| `aarch64-3.10` | основная, физически проверенная |
| `mipsel-3.4` | experimental, частично проверенная |
| `mips-3.4` | experimental |

Для работы нужны Entware в `/opt`, `opkg`, root-доступ и `curl` или `wget`.

Подробная матрица устройств и архитектур: [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md).

## Сообщество

Вопросы, обратная связь, идеи и обсуждение RouterForge — в нашей группе **[Telegram @RouterForge](https://t.me/RouterForge)**.

Ошибки и технические задачи можно оформлять через [GitHub Issues](https://github.com/Fifth-Ace/routerforge/issues).

## Документация

- [Документация RouterForge](docs/README.md)
- [Release Notes 0.9.0](docs/RELEASE_NOTES_0.9.0.md)
- [Установка и обновление](docs/INSTALLATION.md)
- [Архитектура](docs/ARCHITECTURE.md)
- [Модули](docs/MODULES.md)
- [Центр приложений](docs/MARKETPLACE.md)
- [Management v2](docs/MANAGEMENT_V2_API.md)
- [Политика версионирования](docs/VERSIONING.md)
- [Диагностика](docs/TROUBLESHOOTING.md)
- [Changelog](CHANGELOG.md)

## Для разработчиков

Backend: **Go 1.21+**. Frontend: **Node.js 22.x**.

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
go test ./...
go vet ./...
```

Подробнее: [CONTRIBUTING.md](CONTRIBUTING.md).

## Лицензия

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
