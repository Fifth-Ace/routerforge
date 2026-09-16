# RouterForge

[Русский](README.md) | **English**

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.9.0-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.8.5--beta.2-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge 0.9.0** is a compact modular web platform for monitoring, DNS, management and network diagnostics on **Keenetic / Netcraze routers with Entware**.

RouterForge works directly with KeeneticOS/NDMS and Entware capabilities including `ndmc`, RCI, interfaces, routes, DNS, policy routing, processes and services. Users work through one Web UI on **`:2233`** while internal modules connect to Core through Unix sockets.

> [!TIP]
> Core and module versions are independent — see [Versioning Policy](docs/VERSIONING.md).

## Why RouterForge

- **Native Keenetic integration.** RouterForge understands NDMS/KeeneticOS, `ndmc`, RCI, policy routing and router system interfaces.
- **One interface.** Core combines modules, settings, App Center and diagnostics behind a single user-facing Web port: `2233`.
- **Modular design.** Install only the capabilities you need.
- **Guarded operations.** Sensitive actions use constrained server-side contracts, path validation, confirmations and rollback where appropriate.
- **Router-oriented runtime.** Designed for limited CPU, RAM and flash. Node.js is required only to build the frontend, not on the router.
- **Verifiable releases.** Stable is published from a validated exact SHA with multi-arch indexes, SHA256 checks and an immutable release snapshot.

## Stable 0.9.0 components

| Component | Stable | Purpose |
| --- | ---: | --- |
| `routerforge-core` | `0.9.0` | Web UI, authentication, settings, App Center, Registry, release lifecycle and Module ABI |
| `routerforge-dns` | `0.8.1` | DNS/DoT/DoH, resolver control, rolling health, client attribution and diagnostics |
| `routerforge-admin` | `0.8.1` | Processes, Services, File Manager, Maintenance, Entware Terminal and Keenetic NDM Console |
| `routerforge-monitoring` | `0.8.0` | System, Thermal, Storage and Network monitoring |
| `routerforge-network-tools` | `0.9.0` | Network Doctor, Route Inspector, Flow Explorer and Active Probes |
| `routerforge-profiling` | `0.7.1` | local Core profiling |

A fresh installation starts with **Core**. Optional components are installed from App Center when needed.

## Architecture

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

`routerforge-profiling` is separate and is loopback-only by default at `127.0.0.1:6061`.

## Installation

### Stable

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

Open:

```text
http://<router-ip>:2233
```

### Beta

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Stable and Beta are separate rolling channels. Switch deliberately.

## Supported platform

The primary production platform is **Keenetic / Netcraze + Entware**.

| Architecture | Status |
| --- | --- |
| `aarch64-3.10` | primary, physically validated |
| `mipsel-3.4` | experimental, partially validated |
| `mips-3.4` | experimental |

Entware in `/opt`, `opkg`, root access and `curl` or `wget` are required.

See [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md) for the hardware matrix.

## Community

Questions, feedback, ideas and RouterForge discussion: **[Telegram @RouterForge](https://t.me/RouterForge)**.

Bugs and technical tasks can also be filed in [GitHub Issues](https://github.com/Fifth-Ace/routerforge/issues).

## Documentation

- [Documentation index](docs/README.md)
- [Release Notes 0.9.0](docs/RELEASE_NOTES_0.9.0.md)
- [Installation](docs/INSTALLATION.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Modules](docs/MODULES.md)
- [App Center](docs/MARKETPLACE.md)
- [Management v2](docs/MANAGEMENT_V2_API.md)
- [Versioning Policy](docs/VERSIONING.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Changelog](CHANGELOG.md)

## Development

Backend: **Go 1.21+**. Frontend: **Node.js 22.x**.

```sh
sh scripts/build-frontend.sh
sh scripts/build-dns-frontend.sh
sh scripts/build-admin-frontend.sh
sh scripts/build-monitoring-frontend.sh
go test ./...
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
