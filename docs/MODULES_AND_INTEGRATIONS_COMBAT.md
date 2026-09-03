# DNS Monitor — combat-preview modules

## Package boundary

Core stays mandatory and small. Optional functionality is separate:

```text
dns-monitor
├── dns-monitor-admin
├── dns-monitor-system
├── dns-monitor-thermal
├── dns-monitor-storage
├── dns-monitor-network
└── dns-monitor-profiling
```

Core never depends on the optional packages. Every helper module exposes only a
root-owned Unix socket under `/opt/var/run`; no helper opens a LAN TCP port.

### System

`dns-monitor-system`

- per-core rolling CPU usage;
- RAM/swap;
- uptime/load;
- process count.

Socket: `/opt/var/run/dns-monitor-system.sock`

### Thermal

`dns-monitor-thermal`

- `/sys/class/thermal/thermal_zone*`;
- `/sys/class/hwmon/hwmon*/temp*_input`;
- scalar Wi-Fi debugfs temperature files when exported by the driver;
- storage temperature via `smartctl` only when smartctl is already installed.

No smartmontools dependency is pulled automatically.

### Storage

`dns-monitor-storage`

- mount capacity through `statfs`;
- block device model/type/size through sysfs;
- passive read/write B/s and IOPS from `/proc/diskstats`.

No background benchmark workload is generated.

### Network

`dns-monitor-network`

- addresses/MAC/MTU/operstate/speed/duplex;
- RX/TX B/s and packets/s;
- errors/drops;
- `/proc/net/wireless` values when available;
- IPv4 routes;
- conntrack count/max.

### Profiling

`dns-monitor-profiling`

pprof must inspect the Core process itself, therefore this package enables a
Core feature instead of starting a helper.

Default configuration:

```text
listen=127.0.0.1:6061
slow_ms=750
```

Core rejects a non-loopback profiling address. Use SSH port forwarding for
remote pprof. Slow-request logs omit query strings.

## Plain DNS in Core

Core now also discovers ordinary resolver entries from `ndmc -c "show dns-proxy"`
and passively observes UDP/TCP port 53 traffic to those configured resolver
addresses.

Tracked data:

- request/response counts;
- latency and P95;
- DNS errors/NXDOMAIN;
- timeouts;
- profiles/domain bindings from Keenetic discovery;
- recent queries.

A direct client query to the same resolver is marked on the LAN-side packet and
suppressed best-effort when its NATed egress copy is observed. DNS/TCP remains
best-effort for segmented messages, consistent with the existing client capture.

## Marketplace safety

Third-party projects are metadata/adapters only. DNS Monitor does not mirror
their binaries or feeds.

The combat-preview Marketplace remains read-only:

- detection/version/status;
- project link and stable verified UI port where available;
- compatibility hints;
- install-plan preview.

No third-party installer or opkg mutation is executed from the web UI.
