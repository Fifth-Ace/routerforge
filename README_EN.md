# RouterForge

[Русский](README.md) | **English**

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.7.1-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.7.1--beta.4-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge 0.7.1** is a modular web platform for monitoring, DNS diagnostics, management and maintenance on **Keenetic / Netcraze routers with Entware**.

Core owns the shared Web UI, authentication, App Center, package/release lifecycle and Module ABI host. DNS, Management, Monitoring and Profiling are optional packages connected through root-owned Unix sockets. The only user-facing RouterForge TCP listener is **`:2233`**.

> [!IMPORTANT]
> **aarch64-3.10 / ARM64** is the primary fully hardware-validated Stable target.
> `mipsel-3.4` has partial physical validation on Keenetic Giga KN-1010 and remains experimental.
> `mips-3.4` remains experimental without physical hardware validation.
> See [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md).

## Stable 0.7.1 highlights

### Management v2
- Processes and Entware Services with root-session-protected actions;
- **Commander / Explorer** File Manager, tree, volumes, UTF-8 editor, properties and visual chmod;
- guarded mkdir/write/move/delete/chmod inside canonical `/opt` + `/tmp`;
- Maintenance and backup/restore workflows;
- full **Entware Terminal** over WebSocket/PTTY;
- **Keenetic NDM Console** backed by fixed server-side `ndmc`, never a browser-selected executable;
- Entware ↔ Keenetic switching hardware-validated on Keenetic Ultra KN-1812.

### Monitoring
`routerforge-monitoring` replaces the split System/Thermal/Storage/Network packages with **one runtime and one UI** while preserving compatibility sockets/APIs for migration.

### DNS
`routerforge-dns` remains an independent Module ABI v1 runtime with resolver control, verified rollback, client attribution, route-aware diagnostics, and compact internal event rings while preserving **10,000 logical retained events**.

### App Center
RouterForge / Integrations / Entware, guarded lifecycle jobs, preflight metadata, bulk update, exact SHA256 release-index verification and safe local Web UI discovery without blind LAN scans.

Read the full **[RouterForge 0.7.1 Release Notes](docs/RELEASE_NOTES_0.7.1.md)**.

## Official Stable 0.7.1 packages

| Package | Purpose |
| --- | --- |
| `routerforge-core` | Web shell, auth, App Center, Registry/release lifecycle, Module ABI host |
| `routerforge-dns` | DNS runtime/UI/control/observability/diagnostics |
| `routerforge-admin` | Management v2, File Manager, Maintenance, Entware + Keenetic terminals |
| `routerforge-monitoring` | consolidated System/Thermal/Storage/Network runtime + UI |
| `routerforge-profiling` | loopback-only Core profiling |

A fresh bootstrap installs **Core**. Optional capabilities are selected from App Center.

## Quick Stable install

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

Open `http://<router-ip>:2233`.

See [docs/INSTALLATION.md](docs/INSTALLATION.md).

## Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Avoid mixing Stable and Beta unless deliberately switching channels.

## Documentation

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

## Build

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
gofmt -w .
go test ./...
go vet ./...
```

Backend: Go 1.21+. Frontend: Node.js 22.x. Node.js is not required on the router.

## License

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
