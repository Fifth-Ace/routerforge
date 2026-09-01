# DNS Monitor

[Русский](README.md) | **English**

[![Release](https://img.shields.io/github/v/release/Fifth-Ace/dns-monitor?display_name=tag)](https://github.com/Fifth-Ace/dns-monitor/releases)
[![CI](https://github.com/Fifth-Ace/dns-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/Fifth-Ace/dns-monitor/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Keenetic%20%2F%20Netcraze-ARM64-blue)](#requirements)

DNS Monitor provides DNS observability and diagnostics for **Keenetic / Netcraze routers** using the router's native DNS proxy, KeeneticOS/NDMS routing policies and Entware environment.

> [!IMPORTANT]
> **Current supported target: Keenetic / Netcraze + ARM64 (aarch64) only.**  
> DNS Monitor relies on KeeneticOS/NDMS-specific facilities such as `ndmc`, `show dns-proxy`, `show ip hotspot`, `show ip policy`, local DNS proxy listeners and native policy routing. It is **not** a generic OpenWrt/Linux DNS monitor. x86, MIPS and other architectures are currently unsupported.

> [!NOTE]
> This is an independent community project and is not an official Keenetic or Netcraze product.

## Features

DNS Monitor passively observes how the native Keenetic DNS stack handles client requests and presents the data in a lightweight web interface.

- automatically discovers configured DoT/DoH resolvers from Keenetic;
- maps local DNS proxy ports to profile, resolver and protocol;
- watches client DNS requests and router responses;
- correlates client requests with the actual DoT/DoH upstream selected by Keenetic;
- shows DNS activity per device and per LAN/Wi-Fi interface;
- reads device name, MAC/IP, policy, SSID/AP and access path from Keenetic;
- tracks successful answers, DNS errors, timeouts, fallback events and latency;
- detects `CACHE / LOCAL` answers when Keenetic replies without a new upstream request;
- provides per-client drill-down with its own DNS event stream;
- supports pause and search for live DNS streams so events can be inspected without the list constantly moving;
- reads native Keenetic policy routing, including mark/table and available tunnel paths;
- performs policy-aware DoT/DoH diagnostics using the same routing mark as Keenetic;
- compares a failing policy path with the normal default route;
- keeps short rolling history in RAM to avoid unnecessary flash writes.

The web UI listens on **port 2233** by default.

## Requirements

- Keenetic or Netcraze router running KeeneticOS/NDMS;
- **ARM64 / aarch64** CPU;
- Entware mounted at `/opt`;
- root access through Entware SSH;
- standard Keenetic `ndmc` CLI available from the Entware environment.

The current build is developed and tested against the Keenetic ARM64 environment. Other platforms may compile from source, but they are not considered supported and may not provide the Keenetic-specific facilities required by DNS Monitor.

## Quick install

The recommended installation method is to add the DNS Monitor feed to Entware and install the package with `opkg`:

```sh
wget -qO- https://raw.githubusercontent.com/Fifth-Ace/dns-monitor/main/scripts/install-repo.sh | sh
```

The bootstrap script:

1. checks for ARM64 and Entware/opkg;
2. adds `/opt/etc/opkg/dns-monitor.conf`;
3. runs `opkg update`;
4. installs or upgrades the `dns-monitor` package;
5. starts the service.

The web UI is then available at:

```text
http://<router-ip>:2233
```

### Updating

Once the repository is configured, future versions use the normal Entware workflow:

```sh
opkg update
opkg upgrade dns-monitor
```

### Service control

```sh
/opt/etc/init.d/S90dns-monitor stop
/opt/etc/init.d/S90dns-monitor start
/opt/etc/init.d/S90dns-monitor restart
```

Logs:

```sh
tail -f /opt/var/log/dns-monitor.log
```

### Uninstall

Remove the package and repository entry:

```sh
wget -qO- https://raw.githubusercontent.com/Fifth-Ace/dns-monitor/main/scripts/remove-repo.sh | sh
```

### Manual installation from a GitHub Release

If you do not want to add an opkg feed, download the ARM64 archive for the required release, unpack it on the router and run `install.sh`:

```sh
cd /opt/tmp/dns-monitor-v0.1.0
./install.sh
```

## Build from source

Go 1.21+ is required.

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dns-monitor-linux-arm64 .
```

The web frontend is embedded directly into the Go binary. Node.js is not required on the router.

## How it works

DNS Monitor observes both sides of the router DNS path:

```text
Client
  │  UDP/TCP 53
  ▼
Keenetic DNS proxy
  │  localhost:405xx
  ▼
DoT / DoH upstream
```

Passive packet capture is combined with metadata obtained from Keenetic itself. This allows a single event to be represented roughly as:

```text
Gaming-PC
192.168.10.83
Policy2
    ↓
catalog.example.com
    ↓
Google DoT
    ↓
NOERROR / latency / fallback
```

For routing diagnostics, DNS Monitor reads the dynamic `mark` and `table` assigned by Keenetic to a policy and can test an upstream using `SO_MARK`. This makes it possible to distinguish an upstream outage from a failure specific to a policy, VPN or tunnel path.

## Client DNS outcomes

A client-side request is classified as one of:

- `FORWARDED` — a matching local proxy/upstream request was observed;
- `CACHE_LOCAL` — Keenetic replied without a new matching upstream request;
- `ERROR` — the client received a DNS error such as SERVFAIL or REFUSED;
- `CLIENT_TIMEOUT` — no router response was observed before the client timeout.

`CACHE_LOCAL` intentionally combines cached and locally generated answers: passive observation cannot reliably distinguish those cases without modifying the native router DNS proxy.

## Limitations

- DNS-over-TCP capture is currently best-effort when a DNS message fits in the observed TCP segment; full TCP stream reassembly is not implemented yet.
- Direct DoH/DoT used independently by a client is encrypted before reaching the native Keenetic DNS proxy and therefore cannot be attributed as a normal DNS query by this monitor.
- The policy view currently shows available Keenetic tunnel candidates; exact per-flow ECMP tunnel attribution is planned separately.
- Runtime history is memory-only and resets when the daemon restarts.

## API

Useful endpoints include:

```text
/api/health
/api/snapshot
/api/history?minutes=60
/api/quality?minutes=5
/api/fallbacks?minutes=5
/api/error-bursts?minutes=5
/api/clients
/api/client?ip=<client-ip>&limit=500
/api/interfaces
/api/system
```

## Versioning

The public project history starts at **v0.1.0**. Earlier internal development builds used a separate pre-public version sequence and are intentionally not part of the public release history.

## License

DNS Monitor is distributed under the [MIT License](LICENSE).

Copyright © 2026 Fifth-Ace.
