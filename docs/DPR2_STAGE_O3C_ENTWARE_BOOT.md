# DPR2-O3C — inert Entware boot service template

Status: source + CI candidate only. This document is **not** authorization to reboot or activate production routers.

## Motivation

DPR2-O3B is CI+ARM64 DSTest hardware PASS at commit `8e699748155797e19672ae0b7332d76a0b2bab76`.
SIGKILL cleanup retains the durable ledger, and the next manual startup restored both owned DNS zones, verified DNS UDP/TCP, and achieved exact baseline restoration on disable. A real firmware reboot and Entware auto invocation have not been tested.

## Packaging contract

* `routerforge-dns-router` remains a separate experimental optional package; no Stable/Beta/App Center or core changes.
* Service template is packaged ONLY at `/opt/share/routerforge/dns-router/service/S95routerforge-dns-router`, executable but inert.
* The package MUST NOT create anything in `/opt/etc/init.d` or modify Keenetic `opkg initrc`/`dns-override` or perform `system configuration save`.
* To enable boot later, an operator must explicitly copy the template to `/opt/etc/init.d/S95routerforge-dns-router` after reviewing evidence; the service is then invoked by existing Entware `rc.unslung`, when actually configured.
* Service refuses non-DSTest/non-aarch64/non-192.168.10.1 hosts, wrong binary version, missing policy and inaccessible ndmc. It starts the shadow DNS listener and O3B lifetime guardian before restoring ownership.
* Bound to explicit `--ndmc-durable-experimental`, `--ndmc-startup-recovery-experimental` and `--ndmc-lifetime-guard-experimental`; no direct `ndmc` configuration mutations in the init script.
* PID file binds PID and `/proc` starttime, with `/proc/PID/exe` verification before SIGTERM. No arbitrary PID kill and no SIGKILL fallback.
* Limited boot readiness retry (up to 60s) accommodates Entware starting before the LAN/ndmc. On failure the service fails closed and logs diagnostics; it does not change system routes or configuration.

## Hardware acceptance gates (later)

1. Read-only audit of **actual** DSTest `opkg initrc` and `/opt/etc/init.d/rc.unslung` execution; do not assume all KeeneticOS firmware versions automatically invoke it.
2. Manual install of the matching experimental IPK to DSTest only; inspect template in `/opt/share`; verify NO active S95 script and no DNS writes.
3. Manual one-time activation by copying S95, then `start`; verify lifetime guard armed, policy CAS and 2 owned test zones, direct UDP/TCP and ndnproxy unchanged.
4. `stop` must finish independent lifetime cleanup, retain ownership ledger, restore exact baseline and NOT signal foreign processes.
5. Only after approval and a guaranteed out-of-band recovery path, actual reboot, read-only boot evidence, auto-start, rebind and ownership reconciliation, DNS tests. Test absent initrc, missing /opt, and outdated binary failures.
6. Explicit `durable/disable`, `stop`, removal of opt-in S95 file; verify exact DNS inventory, no owned ledger, ndnproxy unchanged, NFQUEUE 300 untouched.

Until all six hardware acceptance gates pass, O3 remains **NOT FINISHED**.
