# RouterForge

[Русский](README.md) | **English**

[![CI](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/routerforge/actions/workflows/ci.yml)
[![Stable](https://img.shields.io/badge/channel-stable-2ea043)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-stable)
[![Beta](https://img.shields.io/badge/channel-beta-d29922)](https://github.com/Fifth-Ace/routerforge/releases/tag/routerforge-beta)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Keenetic%20%2F%20Netcraze-ARM64-blue)](#requirements)

**RouterForge** is a modular web platform for monitoring, DNS diagnostics and router maintenance on **Keenetic / Netcraze ARM64** devices with Entware.

A single Core provides the UI, API, Marketplace, authentication and package lifecycle. Optional capabilities are shipped as independent packages and appear in the UI only when installed.

> [!IMPORTANT]
> The supported target is currently **Keenetic / Netcraze on ARM64 / aarch64** with Entware mounted at `/opt`.
> RouterForge relies on KeeneticOS/NDMS-specific facilities such as `ndmc`, the native DNS proxy and policy routing. It is not intended to be a generic OpenWrt/Linux dashboard.

> [!NOTE]
> RouterForge is an independent community project and is not an official Keenetic or Netcraze product.
> Historical links using the previous repository name remain supported through GitHub redirects and the RouterForge compatibility fallback.

## What RouterForge provides

### Home
- Core and capability state;
- platform summary;
- host telemetry when System/Control is installed;
- Marketplace and Registry state.

### Monitoring
Independently installable official modules:
- **System Monitor** — CPU, RAM/swap, load, uptime and process counters;
- **Thermal Monitor** — thermal/hwmon and available temperature sensors;
- **Storage Monitor** — filesystems, devices and passive I/O telemetry;
- **Network Monitor** — interfaces, addresses, RX/TX, errors/drops, wireless, routes and conntrack.

### DNS
RouterForge DNS keeps the original project's strongest capability:
- plain DNS, DoT and DoH observability;
- per-client and LAN/Wi‑Fi attribution;
- upstream/fallback/timeout/error/latency tracking;
- `CACHE_LOCAL`, `FORWARDED`, `ERROR` and `CLIENT_TIMEOUT`;
- Keenetic policy routing discovery;
- route-aware upstream diagnostics using the policy mark;
- short in-memory history without continuous event writes to flash.

### Control
**RouterForge Control** is a separate read-only helper for inspecting:
- processes;
- listening sockets;
- Entware services;
- installed packages;
- system summary.

The helper uses a root-owned Unix socket and does not open another TCP port.

### Marketplace
- official RouterForge modules;
- detection of supported third-party projects;
- trust/status/compatibility metadata;
- install/update/remove for approved lifecycle plans;
- SHA256 verification before installing RouterForge IPKs;
- independent component versions;
- hourly automatic remote Registry/release-index checks;
- immediate manual **Check for updates**;
- RouterForge batch updates with modules first and Core last.

### Settings and authentication
Authentication is optional and can be enabled from RouterForge Settings:
- credentials use the Entware `root` account;
- password verification reads `/opt/etc/shadow` with `/opt/etc/passwd` fallback;
- sessions live in RAM for 12 hours;
- `HttpOnly` + `SameSite=Strict` cookie;
- configuration: `/opt/etc/routerforge/security.json`.

## Architecture

```text
Browser
   │
   │ http://router:2233
   ▼
RouterForge Core
├── Web UI / REST / SSE
├── Authentication
├── Marketplace + Registry
├── Release index / package lifecycle
├── DNS engine
└── Unix-socket proxy
     ├── RouterForge Control
     ├── System Monitor
     ├── Thermal Monitor
     ├── Storage Monitor
     └── Network Monitor
```

The platform exposes a single web port: **2233**.

## Official packages

| Package | Purpose |
| --- | --- |
| `routerforge-core` | Core, UI, API, Marketplace, auth and release/update logic |
| `routerforge-dns` | enables the DNS capability |
| `routerforge-admin` | RouterForge Control |
| `routerforge-system` | System Monitor |
| `routerforge-thermal` | Thermal Monitor |
| `routerforge-storage` | Storage Monitor |
| `routerforge-network` | Network Monitor |
| `routerforge-profiling` | loopback-only profiling capability |

Components are **versioned independently**. A module version does not need to match the Core version.

## Requirements

- Keenetic or Netcraze with KeeneticOS/NDMS;
- ARM64 / aarch64;
- Entware mounted at `/opt`;
- working `opkg`;
- root access through Entware SSH;
- `sha256sum`;
- `curl` or `wget`.

## Quick install — Stable

Recommended public channel:

```sh
wget -qO- https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-stable/routerforge-stable-bootstrap.sh | sh
```

The bootstrap installs the current **Core + DNS** from the stable release. Their versions are resolved independently from the release index, and each IPK is SHA256-verified before `opkg install`.

Then open:

```text
http://<router-ip>:2233
```

Install other official capabilities from **Marketplace**.

Detailed installation guide: [docs/INSTALLATION.md](docs/INSTALLATION.md).

## Updates

Marketplace is the recommended update path.

RouterForge compares locally installed `opkg` versions with the approved release index for its channel. Remote checks run automatically once per hour; **Check for updates** forces an immediate check.

Release channels:

- `routerforge-stable` is published from `main`;
- `routerforge-beta` is published from `dev` and is marked as a GitHub Pre-release.

Beta installation:

```sh
wget -qO- https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-beta-bootstrap.sh | sh
```

Avoid mixing stable and beta packages unless you deliberately switch channels.

## Service and diagnostics

Core:

```sh
/opt/etc/init.d/S90routerforge restart
```

Log:

```sh
tail -f /opt/var/log/routerforge.log
```

Health:

```sh
wget -qO- http://127.0.0.1:2233/api/health
```

Packages:

```sh
/opt/bin/opkg list-installed | grep '^routerforge-' | sort
```

See [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md).

## Uninstall

```sh
wget -qO- https://raw.githubusercontent.com/Fifth-Ace/routerforge/main/scripts/remove-repo.sh | sh
```

Packages are removed; configuration directories are preserved.

## Documentation

- [Installation and updates](docs/INSTALLATION.md)
- [Modules](docs/MODULES.md)
- [Marketplace and trust model](docs/MARKETPLACE.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Frontend architecture](docs/FRONTEND_ARCHITECTURE.md)
- [Changelog](CHANGELOG.md)
- [Security policy](SECURITY.md)
- [Contributing](CONTRIBUTING.md)

The detailed manuals are currently Russian-first; command examples and architecture references are language-neutral.

## Build from source

Backend: Go 1.21+. Frontend: Node.js 22.x.

```sh
cd frontend
npm install --no-audit --no-fund
npm run check
npm run build
cd ..

gofmt -w .
go test ./...
go vet ./...
```

Production Core is built with `embed_frontend`; Node.js is not required on the router.

## License

[MIT](LICENSE) · Copyright © 2026 Fifth-Ace.
