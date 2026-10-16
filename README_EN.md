<p align="center">
  <img src="docs/assets/routerforge-banner.jpg" alt="RouterForge — monitoring, diagnostics and management for Keenetic / Netcraze with Entware" width="100%">
</p>

# RouterForge

[Русский](README.md) | **English**

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/stable-0.13.0-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/channel-beta-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**RouterForge** is a modular web platform for **Keenetic / Netcraze routers running Entware**. Monitoring, administration, DNS, network diagnostics and security tools are available in one interface.

RouterForge integrates with KeeneticOS/NDMS and Entware through `ndmc`, RCI and system interfaces. The Web UI runs on **`:2233`**; modules connect to Core using local Unix sockets.

> [!NOTE]
> **RouterForge is an independent, non-commercial community project.** It is not an official product, division or partner project of Keenetic, Netcraze or Entware. Trademarks belong to their respective owners. License: MIT.

> [!TIP]
> Core and modules are versioned independently — see [Versioning Policy](docs/VERSIONING.md).

## Installation

You need a Keenetic / Netcraze router running Entware, shell access and an internet connection.

### Stable

For everyday use:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

### Beta

Prerelease channel with new modules and features:

```sh
/opt/bin/opkg update && /opt/bin/opkg install curl && /opt/bin/curl -fsSL https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Open `http://<router-ip>:2233` after installation. **Stable and Beta are separate release channels.** Choose deliberately. See [Installation and updates](docs/INSTALLATION.md).

## Architecture

RouterForge consists of Core and independently installed modules.

```text
Browser
   |
   | http://router:2233
   v
RouterForge Core
├── Web UI / REST / SSE
├── Authentication / Settings
├── App Center / Registry / Releases
├── Module ABI host
└── Unix-socket proxy
     ├── DNS
     ├── Management
     │    ├── Processes / Services
     │    ├── File Manager / Maintenance
     │    ├── Entware Terminal
     │    └── Keenetic NDM Console
     ├── Monitoring
     │    ├── System / Thermal / Storage / Network
     ├── Network Tools
     │    ├── Network Doctor / Route Inspector
     │    └── Flow Explorer / Active Probes
     ├── NFQWS Manager
     │    ├── Diagnostics / AutoSelect
     │    ├── Strategies / Config / Lists
     │    ├── Smart Apply / Backups
     │    ├── DPI Detector
     │    └── NFQWS Menu
     ├── Antiscan Manager
     │    ├── Status / Diagnostics
     │    └── Config / Lists / Recovery
     ├── Port Access Manager
     │    ├── knockd / fwknopd / SPA
     │    └── iptables recent / odin
     └── CrowdSec Manager
          ├── Firewall Bouncer / LAPI
          └── ipset / iptables / Diagnostics
```

`routerforge-profiling` runs separately and is bound to `127.0.0.1:6061` by default. The backend is written in Go; modules do not need public standalone HTTP servers. See [Architecture](docs/ARCHITECTURE.md).

## Why RouterForge

- **Purpose-built for Keenetic / Netcraze.** Designed for KeeneticOS/NDMS and Entware.
- **System integration.** `ndmc`, RCI, interfaces, routes, DNS, policy routing and services.
- **One interface.** Administration, App Center, diagnostics and maintenance through one Web UI.
- **Modular installation.** Core first, optional components as needed.
- **Lightweight runtime.** Go backend, no Node.js runtime on the router.
- **Guarded changes.** Validation, confirmations, backups and rollback where supported.
- **Verifiable releases.** Multi-architecture packages, SHA256 checks and immutable release snapshots.

## RouterForge modules

Available capabilities depend on the installed release channel and package selection.

### Core — the management hub
`routerforge-core`

Web UI, authentication, settings, App Center, updates, module lifecycle and basic device telemetry. **Required** for all other modules.

### DNS — resolver management
`routerforge-dns`

DNS, DoT and DoH configuration, resolver health, latency, errors and fallback; temporarily disable resolvers without removing them. **Use it for** advanced DNS management.

### Management — router administration
`routerforge-admin`

Processes, services, file manager, maintenance, Entware terminal and Keenetic NDM console. **Use it for** routine administration in the browser.

### Monitoring — device overview
`routerforge-monitoring`

CPU, memory, thermals, storage and network interface status. **Use it for** device resource visibility.

### Network Tools — network diagnostics
`routerforge-network-tools`

Network Doctor, Route Inspector, Flow Explorer and Active Probes. **Use it for** WAN, VPN and routing investigations.

### NFQWS Manager — nfqws2 management
`routerforge-nfqws-manager`

Manage an existing `nfqws2-keenetic` installation: configuration, profiles, lists, site diagnostics, strategy selection, test sandbox, guarded apply and backups. **Use it if** you run nfqws2.

### Antiscan Manager — Antiscan management
`routerforge-antiscan-manager`

Manage an installed Antiscan service: configuration, diagnostics, lists, ipset, scheduling and recovery. **Use it if** you run Antiscan.

### Port Access Manager — protected access
`routerforge-port-access-manager` · **Beta**

Access management for network services and forwarded ports through knockd, fwknopd / SPA and iptables recent / odin. **Use it for** port knocking and access control.

### CrowdSec Manager — Firewall Bouncer management
`routerforge-crowdsec-manager` · **Beta**

CrowdSec LAPI connectivity, Firewall Bouncer lifecycle, ipset/iptables diagnostics and block statistics. **Use it if** you deploy CrowdSec protection.

### Profiling — RouterForge diagnostics
`routerforge-profiling`

Local Core profiling for development and performance analysis. **Optional** for advanced diagnostics.

## Supported platforms

RouterForge targets Keenetic / Netcraze with Entware.

| Architecture | Status |
| --- | --- |
| `aarch64-3.10` | Primary, hardware-validated |
| `mipsel-3.4` | Experimental, partly validated |
| `mips-3.4` | Experimental |

Entware in `/opt`, `opkg`, root access and `curl` or `wget` are required. See [Architecture compatibility](docs/ARCHITECTURES.md).

## Community

Ideas, feedback and questions: **[Telegram @RouterForge](https://t.me/RouterForge)**. Bugs and technical requests: [GitHub Issues](https://github.com/Fifth-Ace/routerforge/issues).

## Documentation

- [Documentation](docs/README.md)
- [Latest Stable](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
- [Latest Beta](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
- [All releases](https://github.com/Fifth-Ace/routerforge/releases)
- [Installation](docs/INSTALLATION.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Modules](docs/MODULES.md)
- [App Center](docs/MARKETPLACE.md)
- [Management API](docs/MANAGEMENT_V2_API.md)
- [Versioning](docs/VERSIONING.md)
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
