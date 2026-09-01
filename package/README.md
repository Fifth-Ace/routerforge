# DNS Monitor

DNS observability for **Keenetic / Netcraze routers** using the router's native DNS proxy, routing policies and Entware environment.

> [!IMPORTANT]
> **Current supported target: Keenetic / Netcraze + ARM64 (aarch64) only.**  
> DNS Monitor is built around KeeneticOS/NDMS facilities such as `ndmc`, `show dns-proxy`, `show ip hotspot`, `show ip policy`, local DNS proxy listeners and Keenetic policy routing. It is **not** a generic OpenWrt/Linux DNS monitor. x86, MIPS and other architectures are currently unsupported.

> [!NOTE]
> This is an independent community project and is not an official Keenetic or Netcraze product.

## What it does

DNS Monitor passively observes how DNS is handled by the base router and presents it in a lightweight web interface.

- discovers configured DoT/DoH resolvers from Keenetic automatically;
- maps local DNS proxy ports to profile, resolver and protocol;
- watches client DNS requests and router responses;
- correlates client requests with the actual DoT/DoH upstream selected by Keenetic;
- shows DNS activity per device and per LAN/Wi-Fi interface;
- reads device names, MAC/IP, policy, SSID/AP and access path from Keenetic;
- tracks success, DNS errors, timeouts, fallback events and latency;
- detects `CACHE / LOCAL` answers when Keenetic replies without a new upstream request;
- provides per-client drill-down with a pauseable DNS event stream;
- reads native Keenetic policy routing and exposes policy marks/tables/tunnel paths;
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

The current build has been developed against the Keenetic ARM64 environment. Other platforms may compile from source, but they are not considered supported and are likely to miss required Keenetic-specific facilities.

## Installation

Download or build the ARM64 package, unpack it on the router and run:

```sh
cd /opt/tmp/dns-monitor-v0.1.0
./install.sh
```

The installer places:

```text
/opt/bin/dns-monitor
/opt/etc/init.d/S90dns-monitor
/opt/var/log/dns-monitor.log
```

Open:

```text
http://<router-ip>:2233
```

Service control:

```sh
/opt/etc/init.d/S90dns-monitor stop
/opt/etc/init.d/S90dns-monitor start
```

Logs:

```sh
tail -f /opt/var/log/dns-monitor.log
```

## Build from source

Go 1.21+ is required.

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dns-monitor-linux-arm64 .
```

The web frontend is embedded in the Go binary; Node.js is not required on the router.

## How it works

DNS Monitor observes two sides of the router DNS path:

```text
Client
  │  UDP/TCP 53
  ▼
Keenetic DNS proxy
  │  localhost:405xx
  ▼
DoT / DoH upstream
```

It combines passive packet capture with metadata obtained from Keenetic itself. This lets a single event be presented as, for example:

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

For routing diagnostics, DNS Monitor reads the dynamic policy mark/table assigned by Keenetic and can test the upstream using `SO_MARK`. That makes it possible to distinguish an upstream outage from a failure specific to a Keenetic policy or tunnel path.

## Client outcomes

A client-side request is classified as one of:

- `FORWARDED` — a matching local proxy/upstream request was observed;
- `CACHE_LOCAL` — Keenetic replied without a new matching upstream request;
- `ERROR` — the client received a DNS error such as SERVFAIL/REFUSED;
- `CLIENT_TIMEOUT` — no router response was observed before the client timeout.

`CACHE_LOCAL` intentionally combines cache and locally generated answers: passive observation cannot reliably distinguish those two cases without modifying the router DNS proxy.

## Limitations

- DNS-over-TCP capture is currently best-effort when a DNS message fits in the observed TCP segment; full TCP stream reassembly is not implemented yet.
- Direct DoH/DoT used independently by a client device is encrypted before it reaches the router DNS proxy and therefore cannot be attributed as a normal DNS query by this monitor.
- The policy view shows Keenetic policy/tunnel candidates. Per-flow exact ECMP tunnel attribution is a planned improvement.
- Runtime history is currently memory-only and resets when the daemon restarts.

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

The public GitHub project starts at **v0.1.0**. Earlier internal development builds used a separate pre-public version sequence and are intentionally not part of the public release history.

## License

A public license has not been selected yet. Until one is added, normal copyright rules apply.
