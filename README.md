<p align="center">
  <img src="docs/assets/routerforge-banner.jpg" alt="RouterForge — Monitoring and diagnostics for Keenetic / Netcraze with Entware" width="100%">
</p>

# RouterForge

**Русский** | [English](README_EN.md)

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/channel-stable-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/channel-beta-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Keenetic%20%2F%20Netcraze-ARM64-blue)](#требования)

**RouterForge** — модульная веб-платформа для мониторинга, DNS-диагностики и обслуживания роутеров **Keenetic / Netcraze ARM64** с Entware.

Один Core даёт общий интерфейс, API, Центр приложений, авторизацию и управление жизненным циклом пакетов. Остальные возможности подключаются отдельными пакетами и появляются в интерфейсе только после установки.

> [!IMPORTANT]
> Основная аппаратно проверенная платформа — **Keenetic / Netcraze ARM64 / aarch64** с Entware в `/opt`.
> RouterForge использует KeeneticOS/NDMS-специфичные механизмы (`ndmc`, DNS proxy, policy routing и системные данные роутера) и не позиционируется как универсальная панель для OpenWrt/Linux.
>
> Stable 0.6 также публикует **MIPS / MIPSel как experimental preview**. Эти сборки проходят cross-build/QEMU и runtime compatibility probe, но **не проверены на реальном MIPS/MIPSel-железе**. Установка требует явного experimental opt-in и может быть заблокирована runtime probe. См. [архитектуры](docs/ARCHITECTURES.md).

> [!NOTE]
> RouterForge — независимый community-проект и не является официальным продуктом Keenetic или Netcraze.
> Исторические ссылки на прежнее имя репозитория продолжают работать через GitHub redirect и compatibility fallback RouterForge.

> [!TIP]
> Stable 0.6 baseline: **RouterForge Core 0.6.0**. DNS 0.4.20 и остальные официальные модули версионируются независимо и устанавливаются по необходимости через Центр приложений.

## Что умеет RouterForge

### Главная
- состояние Core и установленных capabilities;
- краткая сводка платформы;
- телеметрия хоста при наличии System/Control;
- состояние Центр приложений и Registry.

### Мониторинг
Официальные модули устанавливаются независимо:
- **System Monitor** — CPU, RAM/swap, load, uptime и процессы;
- **Thermal Monitor** — thermal/hwmon и доступные температурные датчики;
- **Storage Monitor** — файловые системы, устройства и пассивная I/O-телеметрия;
- **Network Monitor** — интерфейсы, адреса, RX/TX, ошибки/drops, Wi‑Fi, маршруты и conntrack.

### DNS
**RouterForge DNS** — самостоятельный Module ABI v1 runtime с собственными backend/API/UI и безопасным управлением нативной DNS-конфигурацией Keenetic:
- plain DNS, DoT и DoH observability;
- вкладки **Обзор / Резолверы / Правила / Трафик / Диагностика**;
- Add/Edit/Delete и временные Disable/Enable для plain DNS, DoT и DoH;
- динамические DHCP/service DNS отображаются read-only;
- один логический multi-domain resolver разворачивается в нужное число нативных записей Keenetic;
- независимые preflight-лимиты **до 8 физических DoT entries и до 8 физических DoH entries**; для plain DNS — максимум **16 доменов на server**;
- каждая мутация проходит `snapshot → mutation → save → readback`; mismatch запускает rollback и его повторную проверку;
- привязка запросов к клиентам и LAN/Wi‑Fi-интерфейсам;
- upstream/fallback/timeout/error/latency, quality windows и health diagnostics;
- `CACHE_LOCAL`, `FORWARDED`, `ERROR`, `CLIENT_TIMEOUT`;
- чтение Keenetic policy routing и route-aware диагностика upstream через policy mark;
- короткая история в RAM без постоянной записи событий на флешку.

### Управление
**RouterForge Control** — отдельный read-only helper для:
- процессов;
- listening sockets;
- Entware services;
- установленных пакетов;
- системной сводки.

Helper работает через root-owned Unix socket и не открывает отдельный TCP-порт.

### Центр приложений
- официальные RouterForge-модули;
- обнаружение поддерживаемых сторонних проектов;
- trust/status/compatibility metadata;
- install/update/remove для разрешённых lifecycle-планов;
- проверка SHA256 перед установкой RouterForge IPK;
- независимые версии каждого RouterForge-компонента;
- автоматическая проверка remote Registry/release-index раз в час;
- немедленная ручная проверка кнопкой **«Проверить обновления»**;
- массовое обновление RouterForge: модули сначала, Core последним.

### Настройки и авторизация
Авторизация включается по желанию в интерфейсе RouterForge:
- используется Entware-пользователь `root`;
- пароль проверяется по `/opt/etc/shadow`, с fallback на `/opt/etc/passwd`;
- сессия хранится в RAM и действует 12 часов;
- cookie `HttpOnly` + `SameSite=Strict`;
- конфигурация: `/opt/etc/routerforge/security.json`.

## Архитектура

```text
Browser
   │
   │ http://router:2233
   ▼
RouterForge Core
├── Web shell / REST / SSE
├── Authentication
├── Центр приложений + Registry
├── Release index / package lifecycle
├── Generic Module API + UI host
└── Unix-socket proxy
     ├── RouterForge DNS
     │    ├── capture / discovery / health
     │    ├── DNS Control + readback / rollback
     │    └── module UI
     ├── RouterForge Control
     ├── System Monitor
     ├── Thermal Monitor
     ├── Storage Monitor
     └── Network Monitor
```

Внешний веб-порт у платформы один: **2233**.

## Официальные пакеты

| Package | Назначение |
| --- | --- |
| `routerforge-core` | Core, UI, API, Центр приложений, auth, release/update logic |
| `routerforge-dns` | независимый DNS runtime, UI, observability, DNS Control и diagnostics |
| `routerforge-admin` | RouterForge Control |
| `routerforge-system` | System Monitor |
| `routerforge-thermal` | Thermal Monitor |
| `routerforge-storage` | Storage Monitor |
| `routerforge-network` | Network Monitor |
| `routerforge-profiling` | loopback-only profiling capability |

Компоненты **версионируются независимо**. Версия Core не обязана совпадать с версиями модулей.

## Требования

- Keenetic или Netcraze с KeeneticOS/NDMS;
- ARM64 / aarch64;
- Entware, смонтированный в `/opt`;
- рабочий `opkg`;
- root-доступ через Entware SSH;
- `sha256sum`;
- `curl` или `wget`.

## Быстрая установка — Stable

Рекомендуемый публичный канал:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

Bootstrap устанавливает актуальный **RouterForge Core** из stable release и проверяет IPK по SHA256 до `opkg install`.

После установки:

```text
http://<ip-роутера>:2233
```

DNS, мониторинг, управление и остальные официальные возможности устанавливаются по необходимости из **Центра приложений**. Уже установленные optional-пакеты bootstrap сам по себе не удаляет.

Подробно: [docs/INSTALLATION.md](docs/INSTALLATION.md).

## Обновления

Основной способ — **Центр приложений**.

RouterForge сравнивает локальные версии `opkg` с утверждённым release-index своего канала. Автоматическая remote-проверка выполняется раз в час; кнопка **«Проверить обновления»** форсирует проверку сразу.

Stable и Beta — разные rolling channels:

- `routerforge-stable` публикуется из `main`;
- `routerforge-beta` публикуется из `dev` и помечен GitHub как Pre-release.

Beta для тестирования:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Начиная с **0.6.0-beta.1**, свежая Beta bootstrap-установка ставит только `routerforge-core`.
DNS, мониторинг, управление и интеграции выбираются после запуска через **Центр приложений**.
Уже установленные optional-пакеты bootstrap сам по себе не удаляет.

Не смешивайте stable и beta пакеты без необходимости.

## Сервис и диагностика

Core:

```sh
/opt/etc/init.d/S90routerforge restart
```

Лог:

```sh
tail -f /opt/var/log/routerforge.log
```

Health:

```sh
wget -qO- http://127.0.0.1:2233/api/health
```

Пакеты:

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

Полный чек-лист: [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md).

## Удаление

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```

Пакеты удаляются, каталоги конфигурации сохраняются.

## Документация

- [Установка и обновление](docs/INSTALLATION.md)
- [Модули](docs/MODULES.md)
- [Центр приложений и модель доверия](docs/MARKETPLACE.md)
- [Архитектура](docs/ARCHITECTURE.md)
- [Поддерживаемые и планируемые CPU-архитектуры](docs/ARCHITECTURES.md)
- [Диагностика](docs/TROUBLESHOOTING.md)
- [Frontend architecture](docs/FRONTEND_ARCHITECTURE.md)
- [Changelog](CHANGELOG.md)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)

## Сборка из исходников

Backend: Go 1.21+. Frontend: Node.js 22.x.

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh

gofmt -w .
go test ./...
go vet ./...
```

Production Core собирается с `embed_frontend`; Node.js на роутере не нужен.

## Лицензия

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
