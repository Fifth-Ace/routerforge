# DNS Monitor Admin module

`dns-monitor-admin` is the first optional DNS Monitor module shipped as a
separate Entware package and helper process.

## Why it is separate

DNS Monitor Core stays focused on DNS observability. Routers that do not need
system administration do not install or run the helper.

The admin helper listens only on:

`/opt/var/run/dns-monitor-admin.sock`

It does not open a TCP port.

DNS Monitor Core proxies read-only `/api/admin/*` requests to the Unix socket.

## Current capabilities

- system summary: hostname, kernel, architecture, uptime, load;
- per-core CPU usage sampled from `/proc/stat`;
- memory and swap from `/proc/meminfo`;
- process table from `/proc`;
- listening TCP/UDP sockets with best-effort PID/process attribution;
- Entware init scripts with non-invasive process-match running detection;
- installed packages from the Entware opkg status database;
- mounted filesystems, capacity and `/proc/diskstats` I/O counters;
- real thermal/hwmon sensors.

## Security boundary

The v1 module is deliberately **read-only**.

Not implemented until authentication/authorization exists:

- shell / terminal;
- reading arbitrary file contents;
- process signals / kill;
- service start/stop/restart;
- opkg install/update/remove;
- configuration writes.

HTTP methods other than GET are rejected by both Core and the helper.

This keeps the initial admin module useful without turning port 2233 into an
unauthenticated root-control interface.
