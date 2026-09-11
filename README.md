# RouterForge

**Русский** | [English](README_EN.md)

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.7.1-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.7.1--beta.4-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge 0.7.1** — модульная веб-платформа для мониторинга, DNS-диагностики, управления и обслуживания роутеров **Keenetic / Netcraze с Entware**.

Core предоставляет Web UI, авторизацию, Центр приложений, package/release lifecycle и Module ABI host. DNS, Management, Monitoring и Profiling подключаются отдельными пакетами через root-owned Unix sockets. Единственный пользовательский RouterForge TCP listener — **`:2233`**.

> [!IMPORTANT]
> **aarch64-3.10 / ARM64** — основной полностью аппаратно проверенный Stable target.
> `mipsel-3.4` имеет partial physical validation на Keenetic Giga KN-1010 и остаётся experimental.
> `mips-3.4` остаётся experimental без физической hardware validation.
> См. [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md).

## Stable 0.7.1 — главное

### Management v2
- Processes и Entware Services с root-session protected actions;
- File Manager: **Commander / Explorer**, tree, volumes, UTF-8 editor, properties, visual chmod;
- guarded mkdir/write/move/delete/chmod в canonical `/opt` + `/tmp` boundary;
- Maintenance и backup/restore workflows;
- полноценный **Entware Terminal** по WebSocket/PTTY;
- **Keenetic NDM Console**: backend запускает фиксированный server-side `ndmc`, а не executable из браузера;
- переключение Entware ↔ Keenetic аппаратно проверено на Keenetic Ultra KN-1812.

### Monitoring
`routerforge-monitoring` объединяет прежние System/Thermal/Storage/Network packages в **один runtime и один UI**, сохраняя compatibility sockets/API для штатной migration.

### DNS
`routerforge-dns` — отдельный Module ABI v1 runtime:
- plain DNS / DoT / DoH observability;
- resolver Add/Edit/Delete/Disable/Enable;
- snapshot → mutation → save → readback → verified rollback;
- client/LAN/Wi-Fi attribution;
- fallback/timeout/error/latency diagnostics;
- policy-routing-aware upstream diagnostics;
- compact internal event rings с сохранением логической retention depth **10 000**.

### Центр приложений
- RouterForge / Integrations / Entware;
- guarded install/update/remove jobs;
- preflight, dependencies, sizes;
- bulk update с Core последним;
- exact release-index + SHA256;
- Generic Runtime Web UI Discovery без blind LAN scan;
- fail-closed SSRF/XFO/CSP/redirect boundary.

Большой патчноут: **[RouterForge 0.7.1 Release Notes](docs/RELEASE_NOTES_0.7.1.md)**.

## Архитектура

```text
Browser -> Core :2233
           |
           +-- routerforge-dns        (Unix socket)
           +-- routerforge-admin      (Unix socket)
           |    +-- File Manager / Maintenance
           |    +-- Entware Terminal
           |    `-- Keenetic NDM Console
           `-- routerforge-monitoring (Unix socket)
                +-- System
                +-- Thermal
                +-- Storage
                `-- Network
```

Profiling — optional Core capability, по умолчанию только `127.0.0.1:6061`.

## Официальные пакеты Stable 0.7.1

| Package | Назначение |
| --- | --- |
| `routerforge-core` | Web shell, auth, App Center, Registry/release lifecycle, Module ABI host |
| `routerforge-dns` | DNS runtime/UI/control/observability/diagnostics |
| `routerforge-admin` | Management v2, File Manager, Maintenance, Entware + Keenetic terminals |
| `routerforge-monitoring` | consolidated System/Thermal/Storage/Network runtime + UI |
| `routerforge-profiling` | loopback-only Core profiling |

Fresh bootstrap ставит **Core**; optional capabilities выбираются через Центр приложений.

## Установка Stable

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

После установки: `http://<ip-роутера>:2233`

Подробно: [docs/INSTALLATION.md](docs/INSTALLATION.md).

## Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Stable и Beta не следует смешивать без осознанной смены channel.

## Диагностика

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
wget -qO- http://127.0.0.1:2233/api/health
ls -l /opt/var/run/routerforge-*.sock 2>/dev/null
```

См. [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md).

## Документация

- [Documentation index](docs/README.md)
- [Release Notes 0.7.1](docs/RELEASE_NOTES_0.7.1.md)
- [Installation](docs/INSTALLATION.md)
- [Modules](docs/MODULES.md)
- [Management v2](docs/MANAGEMENT_V2_API.md)
- [File Manager API](docs/MANAGEMENT_V2_FILES_API.md)
- [App Center](docs/MARKETPLACE.md)
- [Architecture](docs/ARCHITECTURE.md)
- [CPU architectures](docs/ARCHITECTURES.md)
- [Monitoring migration](docs/MONITORING_MIGRATION.md)
- [Release process](docs/RELEASE_PROCESS.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Changelog](CHANGELOG.md)
- [Security](SECURITY.md)
- [Contributing](CONTRIBUTING.md)

## Сборка

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
gofmt -w .
go test ./...
go vet ./...
```

Backend: Go 1.21+. Frontend: Node.js 22.x. Node.js на роутере не требуется.

## Лицензия

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
