# DPR2-O2 — opt-in durable zone activation / deactivation (feature-only)

Base: `4a2fa1cc8525cec1b7b194319f20daea758fcced` (DPR2-O1 CI PASS).

O2 implements a manually enabled, DSTest-only lifecycle for at most eight
user-defined non-overlapping DNS zones. It extends the O1 ownership ledger
rather than inferring ownership from matching `ndmc` entries.

## Persistence and transaction order

**Activation**

1. Check exact revision, plan confirmation token, active LAN listener, no
   ownership ledger, no foreign/overlapping zone, and exact DSTest host.
2. Take canonical running DNS snapshot and atomically write `pending` ownership
   ledger including original snapshot and SHA256 before any `ndmc add`.
3. Arm an independent reverse-on-crash watchdog (DPR2-N worker).
4. Add only planned zones; verify exact registration and actual DNS forwarding.
5. Atomically change ledger to `active` with CAS and acknowledge watchdog.

On failure, restore exact baseline. If restoration cannot be confirmed,
retain `pending` and the independent watchdog rather than reporting success.

**Deactivation**

1. Require the precise config CAS and ownership-ledger CAS; confirm full
   current DNS inventory consists only of baseline plus owned registrations.
2. Atomically mark `deactivating` before deleting anything.
3. Arm an independent *reverse* watchdog which recreates only missing owned
   registrations if the manager dies during removal.
4. Remove registered owned zones and check exact original DNS snapshot.
5. Acknowledge reverse watchdog BEFORE deleting the ownership ledger.

On mid-operation failure, attempt to restore the pre-deactivation inventory and
return ledger to `active`. If either fails, preserve recoverable journal and
report an ambiguous state. No foreign registration is intentionally removed.

## CLI / API safety gates

The feature is disabled by default. It requires all three explicit flags:
`--serve --shadow-forward --shadow-lan-experimental` plus the additional
`--ndmc-durable-experimental`, and exact DSTest hostname/address checks.

Unix API (all POST + action header + JSON and revision match):

- `/v1/keenetic/durable/prepare` — action `durable-prepare`, `If-Match`.
- `/v1/keenetic/durable/apply` — action `durable-apply`, `If-Match`, plan token.
- `/v1/keenetic/durable/disable` — action `durable-disable`, `If-Match`,
  `X-RouterForge-Ledger-Match`.

An O1 ownership-status endpoint remains read-only and detects pending or
`deactivating` recovery states. Requests do not automatically install DNS.

## Deferred to DPR2-O3

No boot-time auto-restore/reconciliation, no init script, no system configuration
save and no release/promotion. O2 must not be considered production-ready on a
router that may reboot or stop the DNS forwarder; this requires the O3 startup
and shutdown lifecycle safety gate plus dedicated DSTest hardware testing.

## CI acceptance

- Existing complete RouterForge CI + P15 shared safety contract;
- `go test`, `go test -race`, `go vet` and O2 negative tests;
- experimental aarch64 / mips / mipsel IPK package matrix;
- only DNS Router payload, script and documentation modified;
- no hardware access, runtime mutation or stable/beta release in O2 CI.
