# RouterForge DNS Router — DPR2-M (bounded on-device ndmc trial controller)

**Target only:** `feature/dns-router-r7-stage-m`. **Exact base:** Stage L commit `6df7db11b1ab92d04e254b8cdeca96bb6086ecab` / CI `40596233365` SUCCESS. The prior DSTest DPR2-I/J/K hardware acceptance remains valid; no Stage M physical hardware acceptance yet.

## Purpose

Move the already hardware-proven domain split transaction from an ad hoc laboratory script into the DNS Router's internal API, keeping a strict **DSTest-only, test-zone-only, always-rollback** scope. This is a one-shot proof, **not permanent DNS service activation**.

## Explicit runtime permissions

1. Binary requires `--serve --shadow-lan-experimental --ndmc-transaction-experimental`. The new API is 503 unless that capability is explicitly enabled at process startup.
2. Host gate rejects anything other than `DSTest` with `192.168.10.1` among its actual addresses.
3. Config must be CAS-pinned and already activated on the exact LAN listener `192.168.10.1:55358`; one rule only, suffix `dpr2m.test`, exactly one upstream `192.168.10.1:55354`. Existing non-DPR2M upstream rules cannot be applied through this endpoint.
4. Request: `POST /v1/keenetic/domain-trial`, header `X-RouterForge-Action: ndmc-trial`, header `If-Match: <exact revision>`, body `{"operation":"trial"}` via the private Unix socket management API. Without the action header: 403; missing capability: 503; stale revision: 409.
5. Read baseline `ndmc -c 'show ip name-server'`, require JSON server[] and absence of any `dpr2m.test` entry. Refuse to change a pre-existing matching domain.
6. Arm separate OS process `/proc/self/exe --ndmc-rollback-worker <private journal>` with 115-second bounded watchdog, before invoking any DNS modification. It survives parent process loss using independent process session, and only knows the exact test-zone removal command. Journal and result file remain for diagnostics if recovery is ambiguous.
7. Add only `ip name-server 192.168.10.1:55358 dpr2m.test`, verify it appears in ndmc's DNS inventory, then send real UDP and TCP DNS requests to both localhost:53 and 192.168.10.1:53; require synthetic A `203.0.113.42` for `probe.dpr2m.test`.
8. ALWAYS remove the test entry with `no ip name-server 192.168.10.1:55358 dpr2m.test` before success; verify original DNS inventory exactly restored before telling watchdog it can exit and deleting its journal. Any inability to verify rollback yields `ambiguous` and leaves watchdog armed.
9. Resource safety: mutex, OS flock, bounded ndmc calls, bounded network probes, no config save, firewall, NFQUEUE or install operations.

## CI contract

- Unit tests cover normal transaction, stale revision, inactive listener, already existing zone, unexpected config, partial ndmc add, probe failure, failed rollback and API permission gates.
- A real independent watcher routine is tested using an ephemeral executable `ndmc` mock; it removes the exact journal-owned test zone when parent PID is missing.
- Existing Go tests and race checks preserved; experimental ARM64/MIPS/MIPSEL IPK build matrix updates to `0.14.0~dev.dpr2m`.
- CI never executes real ndmc on a router, nor starts a shadow listener on any Keenetic device.

## Hardware validation after CI

Stage M needs a **separate hardware acceptance** on DSTest using real `ndmc`, the exact compiled ARM64 binary, the temporary lab upstream and the management Unix socket; this archive only pushes source+CI. We will keep the previous K hardware result as retained and report Stage M hardware as NOT_RUN. Production enablement and arbitrary domain zones remain forbidden.
