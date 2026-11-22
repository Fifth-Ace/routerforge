# DPR2-O3 — Explicit startup recovery, source gate

Base: `a9d9a846e6d0dd6d9ff832e3c2298061ef21cedd` (O2 CI + DSTest hardware PASS).

## Implemented

- Startup capability is **disabled by default** and requires four explicit CLI flags: `--serve --shadow-lan-experimental --shadow-forward --ndmc-durable-experimental` plus `--ndmc-startup-recovery-experimental`.
- Runs only behind the existing strict DSTest host gate. No global resolver, port 53, DHCP, NFQUEUE or native configuration save.
- Reads canonical native DNS snapshot and exact ownership ledger; never adopts observed zones without saved ledger.
- Requires exact policy revision, same managed LAN listener, exact zone set, active runtime listener, no competing watchdog journal, and unchanged non-owned DNS inventory.
- For saved **active** ledger, re-registers missing zones after a reboot or daemon restart under independent DPR2-N rollback watchdog; verifies exact ownership and UDP/TCP forwarding. Returns an explicit status.
- Repeated restart with all zones already present is idempotent and read-only.
- `pending`, `deactivating`, stale, foreign and malformed ledger states are blocked (fail-closed); no implicit correction or adoption.
- Read-only `GET /v1/keenetic/startup/status` reports the reconciliation state when enabled; HTTP 503 when capability is off.
- Source-only and CI-only acceptance; hardware reboot/kill is intentionally excluded.

## OPEN SAFETY ITEMS — NOT RELEASE READY

1. Implement and hardware-test independent **lifetime watchdog** for a daemon that owns active DNS zones; current transaction watchdog only covers a bounded reapply.
2. Complete and test explicit recovery of `pending` and `deactivating` ledger states after power loss without adopting foreign zones.
3. Provide optional Entware startup service with explicit opt-in (none installed yet), boot order and graceful shutdown behavior to prevent DNS blackholes.
4. Test normal restarts, SIGKILL and *real* reboot only after an independent failsafe can remove exact owned entries.
5. Final UI/CORE integration and packaging are still DPR2-P.

**Do not run this experimental boot mode on a working router.** Do not count CI or simulated reboot as hardware acceptance. Main/dev/beta/stable remain untouched.
