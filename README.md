<p align="center">
  <img src="docs/assets/routerforge-banner.jpg" alt="RouterForge — мониторинг, диагностика и управление Keenetic / Netcraze с Entware" width="100%">
</p>

# RouterForge

**Русский** | [English](README_EN.md)

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.9.0-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.8.5--beta.2-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge 0.9.0** — компактная модульная веб-платформа для мониторинга, DNS, управления и сетевой диагностики роутеров **Keenetic / Netcraze с Entware**.

RouterForge использует возможности KeeneticOS/NDMS и Entware напрямую: `ndmc`, RCI, интерфейсы, маршруты, DNS, policy routing, процессы и службы. Пользователь работает через единый Web UI на **`:2233`**, а внутренние модули подключаются к Core через Unix sockets.

> [!TIP]
> Core и модули версионируются независимо — [Versioning Policy](docs/VERSIONING.md).

## Почему RouterForge

- **Нативная работа с Keenetic.** Не просто Linux-панель: RouterForge понимает NDMS/KeeneticOS, `ndmc`, RCI, policy routing и системные интерфейсы роутера.
- **Один интерфейс.** Core объединяет модули, настройки, Центр приложений и диагностику; внешний Web-порт платформы один — `2233`.
- **Модульность.** Нужные функции ставятся отдельными пакетами, без обязательной установки всего набора.
- **Безопасные операции.** Опасные действия проходят через ограниченные server-side контракты, проверки путей, подтверждения и rollback там, где он нужен.
- **Подходит для роутера.** Runtime рассчитан на ограниченные CPU/RAM/flash; Node.js нужен только для сборки фронтенда и не требуется на устройстве.
- **Проверяемые релизы.** Stable публикуется из заранее проверенного exact SHA с multi-arch индексами, SHA256 и immutable release snapshot.

## Из чего состоит Stable 0.9.0

| Компонент | Stable | Назначение |
| --- | ---: | --- |
| `routerforge-core` | `0.9.0` | Web UI, авторизация, настройки, Центр приложений, Registry, release lifecycle и Module ABI |
| `routerforge-dns` | `0.8.1` | DNS/DoT/DoH, управление резолверами, rolling health, клиентская атрибуция и диагностика |
| `routerforge-admin` | `0.8.1` | Processes, Services, File Manager, Maintenance, Entware Terminal и Keenetic NDM Console |
| `routerforge-monitoring` | `0.8.0` | System, Thermal, Storage и Network monitoring |
| `routerforge-network-tools` | `0.9.0` | Network Doctor, Route Inspector, Flow Explorer и Active Probes |
| `routerforge-profiling` | `0.7.1` | локальное профилирование Core |

Свежая установка начинается с **Core**. Остальные компоненты устанавливаются по необходимости через Центр приложений.

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

После установки откройте:

```text
http://<ip-роутера>:2233
```

### Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Stable и Beta — разные rolling channels. Переключайте канал осознанно.

## Поддерживаемая платформа

Основная production-платформа — **Keenetic / Netcraze + Entware**.

| Архитектура | Статус |
| --- | --- |
| `aarch64-3.10` | основная, физически проверенная |
| `mipsel-3.4` | experimental, частично проверенная |
| `mips-3.4` | experimental |

Для работы нужны Entware в `/opt`, `opkg`, root-доступ и `curl` или `wget`.

Подробная матрица устройств и архитектур: [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md).

## Сообщество

Вопросы, обратная связь, идеи и обсуждение RouterForge — в нашей группе **[Telegram @RouterForge](https://t.me/RouterForge)**.

Ошибки и технические задачи также можно оформлять через [GitHub Issues](https://github.com/Fifth-Ace/routerforge/issues).

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
