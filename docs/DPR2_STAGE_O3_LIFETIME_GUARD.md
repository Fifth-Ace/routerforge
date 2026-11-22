# DPR2-O3B: independent lifetime guardian (source+CI gate)

Base: `764695c1a5256f7c9d6464c4b0706d01b223f932` (`feature/dns-router-r7-stage-o3`).

## Functionality

- DSTest-only, explicit `--ndmc-lifetime-guard-experimental` (requires durable + startup experimental flags). No autostart and no OS configuration save.
- Independent process launched **before** startup reconciliation, verifies parent PID **and** `/proc/<pid>/stat` start time, and checks the exact TCP listener on `192.168.10.1:55358`.
- After process death or three consecutive missing-listener checks, waits for short transaction watchdogs, confirms their actual PASS results, then snapshots `ndmc` and removes only owned DNS zones using the existing exact-baseline rollback machinery.
- Foreign owners, overlapping zones, or inventory drift cause a **hard FAIL with preserved journal**; no unowned DNS changes.
- Leaves the ownership ledger intact, so the next explicitly enabled start can restore verified zones.
- Closed successful guard records are moved to diagnostic history under `/tmp/rf-dpr2o3-completed-*`; failed records remain blocking future startup. No automatic retry.
- API refuses `shadow/control stop`, `shadow/control apply`, and config overwrite whenever a durable ownership ledger exists.

## Safety and next acceptance

This code is **not accepted on actual hardware** by source CI alone. Real hardware must verify `SIGKILL`, listener failure, simultaneous transaction watchdog, interprocess blocking, exact DNS baseline, and no `ndnproxy` restart or NFQUEUE changes. Entware init/autostart is **not** installed by this stage; it is a separate, opt-in step after passing hardware checks. Never run these crash tests on production routers.
