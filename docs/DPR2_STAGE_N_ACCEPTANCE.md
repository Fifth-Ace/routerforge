# DPR2-N — temporary multi-zone DNS transaction (source + CI)

## Scope

This phase extends the **hardware-verified DPR2-M FIX4** base to **1–8 user-configured suffix zones**.
A separate explicit `--ndmc-managed-trial-experimental` capability, together with `--serve --shadow-lan-experimental`, is required. The transaction is restricted to the DSTest router (`192.168.10.1`) until dedicated ARM64 hardware acceptance.

## API

- `POST /v1/keenetic/domain-managed/prepare` with `X-RouterForge-Action: managed-prepare`, JSON `{"operation":"prepare"}`, and `If-Match` CAS revision: read-only inventory, collision/overlap rejection, SHA256 confirmation token bound to revision, ordered plan, and exact `ndmc` baseline.
- `POST /v1/keenetic/domain-managed/trial` with `X-RouterForge-Action: managed-trial`, JSON `{"operation":"trial","token":"sha256:..."}`, and the same `If-Match`: recompute inventory and token before any mutation, add only selected zones, verify forwarding from `ndnproxy` on 127.0.0.1/192.168.10.1 via UDP/TCP and forwarder request counters, then **remove in reverse order and verify exact baseline**.

## Safety

- Saved configuration and active runtime revision must match; require explicit opt-in flag, private IPv4 LAN listener and exact client ACL.
- No names that collide with current registered DNS zones. No protected local names, TLD-only zones, exact/subdomain-only rules, more than 8 zones, or source/port recursion.
- Cross-process `flock` shared with DPR2-M. Separate watchdog process uses its own lease journal, a 115-second deadline, a 65-second active-operation limit with a 35-second rollback budget, and exact baseline comparison after recovery.
- A failure after partial addition must attempt rollback and return non-PASS; the watchdog remains active if exact restoration cannot be confirmed.
- **No automatic or persistent apply**. No `system configuration save`, firewall changes, NFQUEUE modification, `ndnproxy` restart, port 53 listener or modifications to other routers.

## Offline acceptance

- `go test ./modules/dns-router-runtime/...`, `go test -race`, `go vet`
- Managed N tests for two-zone transaction, exact rollback, injected add/remove failures, collisions and ancestors, stale CAS/token, changed DNS inventory and API method/action gates.
- UI JS syntax checks and experimental IPK builds for aarch64/mips/mipsel.

## Next required gate

The actual multi-zone `ndmc` transaction and independent watchdog MUST be tested on DSTest **after the CI passes**. Only then can an opt-in persistent, restart-resilient configuration application be considered. Do not infer hardware readiness or durable activation from offline CI.
