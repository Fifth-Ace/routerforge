# DPR2-O1 · ownership ledger and read-only state reconciliation

Base: `5ae957256ccfb4428355cbeaa5c10d3a1d1dbd89` (DPR2-N, CI `40685158180`, hardware FIX2 PASS).

The repository remains in opt-in experimental DNS Router mode. This feature branch introduces a **future persistent activation prerequisite**, not persistent activation.

## Implemented

- A small (≤32 KiB), strict-schema ownership ledger (`schema_version=1`) with CAS digest, atomic replacement and directory sync. State is either `pending` or `active`. The ledger path is `<config-path>.managed-state.json` and is not created by the O1 service.
- Read-only reconciliation reports missing zones, foreign/overlapping registrations, `pending_recovery`, `stale_policy`, and `verified_active` only when a valid active ledger and matching live DNS records exist.
- New API: `GET /v1/keenetic/managed-ownership/status`. `POST` is forbidden. In the absence of a ledger, the call returns `disabled` **without invoking ndmc**.
- New read-only UI panel. No button for persistent apply, delete or autostart exists.
- DSTest only: a ledger can only claim `192.168.10.1:55358` and at most eight validated non-overlapping zones. Support for other routers is **not** enabled.

## Next stages (not implemented by O1)

1. O2: confirm + pending write ahead of ndmc add; exact owned mutation and rollback, cancellation and recovery.
2. O3: boot service, monotonic recovery state machine, reboot/kill testing on DSTest with preserved native DNS and timeout budgets.
3. P: UI/Core integration, packaging, MIPS/MIPSEL hardware/CI acceptance and release gating.

## Safety properties

- O1 does not call `ndmc` to mutate anything, does not invoke system configuration save and does not start/restart system services.
- No foreign zone is adopted based on domain/IP match without a valid ledger; missing records are not automatically installed, and drift is not automatically repaired.
- A corrupted or unknown state file fails closed and preserves the file for manual investigation.
- Main/dev/beta/stable and all routers are not modified by this CI-only delivery.
