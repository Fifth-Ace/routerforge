# RouterForge

[Русский](README.md) | **English**

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.9.0-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/beta-0.8.5--beta.2-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge 0.9.0** is a modular web platform purpose-built for **Keenetic / Netcraze routers with Entware**: monitoring, DNS, management, network diagnostics and maintenance in one interface.

RouterForge works directly with KeeneticOS/NDMS and Entware capabilities including `ndmc`, RCI, interfaces, routes, DNS, policy routing, processes and services. Users work through one Web UI on **`:2233`** while internal modules connect to Core through Unix sockets.

> [!NOTE]
> **RouterForge is an independent, non-commercial community project.** It is not an official product, division or partner project of **Keenetic, Netcraze, Entware** or any other company or project mentioned here. All trademarks belong to their respective owners. RouterForge is licensed under MIT.

> [!TIP]
> Core and module versions are independent — see [Versioning Policy](docs/VERSIONING.md).

## Why RouterForge

- **Purpose-built for Keenetic / Netcraze routers.** RouterForge is not a generic Linux panel adapted to a router; its architecture, runtime and UI are designed around KeeneticOS/NDMS and Entware from the start.
- **Deep router integration.** RouterForge works with `ndmc`, RCI, policy routing, routes, interfaces, DNS and router services instead of exposing only generic Linux metrics.
- **One interface instead of multiple disconnected panels.** Core brings modules, settings, App Center, diagnostics and maintenance together on one user-facing Web port: `2233`.
- **Modular installation.** Start with Core and add only the capabilities you actually need.
- **Lean runtime.** Go backend, bounded caches/history, no Node.js on the router and no unnecessary external HTTP services.
- **Guarded operations.** Sensitive actions use constrained server-side contracts, path validation, confirmations and rollback where it actually matters.
- **Verifiable release chain.** Stable is published from a validated exact SHA with multi-arch indexes, SHA256 checks and an immutable release snapshot.

## RouterForge modules — Stable 0.9.0

### Core — one control point
`routerforge-core` · **0.9.0**

RouterForge itself: the shared Web UI, authentication, settings, App Center, Registry and the host for every other module.

**Everyone needs it:** Core is the platform foundation. Installation starts here and the rest of RouterForge is managed through it.

### DNS — more control than stock DNS settings
`routerforge-dns` · **0.8.1**

More than a list of DNS servers. The module gives DNS its own workspace: classic DNS, DoT and DoH, resolver management, current health, latency, errors, fallback and a clear explanation of **what is unhealthy right now and why**.

Unlike stock DNS management, RouterForge lets you **temporarily disable a resolver without deleting it**, then enable it again later. That makes testing, provider comparison and temporary isolation of an unstable resolver much easier without losing its configuration. Dynamic system resolvers are protected from accidental editing.

It also:
- shows health **per resolver**, not only as one global status;
- uses a rolling health window so old errors do not keep a resolver degraded forever;
- treats normal `NXDOMAIN` responses as informational instead of a health failure;
- shows latency/p95, errors, fallback and outage state;
- helps reveal which resolver client traffic is actually using;
- applies changes through a guarded `snapshot → validation → mutation → save → readback → rollback` flow.

**Useful if:** you want to do more than simply “set DNS” — you want to **control it, compare resolvers and quickly understand failures**, especially with multiple DNS providers, DoT/DoH or more complex network policy.

### Management — administration without living in SSH
`routerforge-admin` · **0.8.1**

Processes and Entware services, File Manager, Maintenance, full Entware Terminal and Keenetic NDM Console in one UI.

**Useful if:** you want most day-to-day administration in the browser — inspect a process, restart a service, edit a file, open a shell or enter `ndmc` without jumping between separate tools.

### Monitoring — the router's health at a glance
`routerforge-monitoring` · **0.8.0**

System, Thermal, Storage and Network in one place: CPU load, memory, temperatures, storage and interface state.

**Useful if:** you want one page that answers “is the router healthy?” without opening `top`, `df`, `ip` or hunting through thermal sensors manually.

### Network Tools — see where traffic is really going
`routerforge-network-tools` · **0.9.0**

Network Doctor, Route Inspector, Flow Explorer and Active Probes: routing, active flows, reachability and path diagnostics in one module.

**Useful if:** you have multiple WAN/VPN links, policy routing, complex routes, or regularly ask “why is this destination going that way?”.

### Profiling — RouterForge internals
`routerforge-profiling` · **0.7.1**

Local Core profiling for development and deep performance diagnostics.

**Most users do not need it:** install only when you specifically need profiling.

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

`routerforge-profiling` is separate and loopback-only by default at `127.0.0.1:6061`.

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

RouterForge is developed for **Keenetic / Netcraze routers with Entware**.

| Architecture | Status |
| --- | --- |
| `aarch64-3.10` | primary, physically validated |
| `mipsel-3.4` | experimental, partially validated |
| `mips-3.4` | experimental |

Entware in `/opt`, `opkg`, root access and `curl` or `wget` are required.

See [docs/ARCHITECTURES.md](docs/ARCHITECTURES.md) for the hardware matrix.

## Community

Questions, feedback, ideas and RouterForge discussion: **[Telegram @RouterForge](https://t.me/RouterForge)**.

Bugs and technical tasks can be filed in [GitHub Issues](https://github.com/Fifth-Ace/routerforge/issues).

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
