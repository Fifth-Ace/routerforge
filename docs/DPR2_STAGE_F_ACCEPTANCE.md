# RouterForge DNS Router — DPR2-F Hardware Evidence & DNS Proxy Integration Design

Status: feature-stage design and repeatable read-only evidence tooling. NOT a hardware acceptance and NOT a DNS takeover.

## Exact ancestry

DPR2-E: `2c61734188ba867d6a5e45b76acb33a463a775d6` (`40560645084` successful). This stage changes only DNS Router source, standalone CI verification and scoped documentation. Router runtime on real Keenetic remains unchanged until a separately reviewed, explicit future activation.

## Delivered as one cohesive stage

- `GET /api/modules/dns-router/keenetic/integration-plan` (via Core module proxy) returns a structured list of non-activating adapter candidates, uncertainty and test gates.
- `GET /api/modules/dns-router/hardware/evidence` returns local resource evidence, passive Keenetic detection and the design plan. POST returns HTTP 405.
- Standalone `routerforge-dns-router --hardware-report` prints read-only JSON without opening listeners, loading or writing DNS config or invoking Keenetic configuration commands.
- `scripts/dpr2-f-hardware-readonly.sh` is an independent BusyBox/POSIX-shell baseline script for *a consciously selected test router over SSH*. Runs without installation; reads `/proc/meminfo`, `/proc/net/*`, process VmRSS, `/etc/resolv.conf`, `df`, and bounded `ndmc -c 'show ...'` only when `timeout` is available. It does not execute DNS queries.
- User interface has two explicit read-only report buttons. No background polling, write APIs, intercept, port 53 listener or routing modification.
- Deterministic tests assert there is **no activation permission** even when ndmc and port 53 are visible. Three experimental IPKs and Core proxy gates remain part of the existing CI.

## Integration choices — gated, not automatically selected

| Candidate | Status | Required evidence before implementation |
|---|---|---|
| Manual shadow on loopback/high port | Available from DPR2-E, user-started | Current loopback smoke pass; no DNS mutation |
| Keenetic DNS Proxy forwarding to explicit high port | Research only | Exact KeeneticOS firmware support; recursion prevention; local-name preservation; IPv4/IPv6 and boot tests; full rollback |
| Explicit opt-in client DNS | Research only | Interfaces/ACL and public exposure review, full UDP/TCP tests |
| `opkg dns-override`, iptables REDIRECT, DNS takeover | Not approved | Separate architecture decision and exact preimage/rollback acceptance |

### Hypothesis to confirm rather than assume

The Keenetic CLI reference documents `show ip name-server`; selected versions also accept `ip name-server` variants. Whether the target firmware accepts `IP:port` as a global DNS Proxy upstream and correctly respects multiple domains, local zones, DoT/DoH priority and link policies must be *measured on DSTest*. Do not infer compatibility from a process existence check or the port 53 socket table.

### Required tests on real target (READ-ONLY stage)

1. Identify firmware, model, architecture and installed Entware without mutating config.
2. Capture which processes own DNS-related listeners. Proc tables alone are not process ownership proof.
3. Determine `opkg dns-override` state from explicitly reviewed device outputs, never assume it is off.
4. Baseline DNS behavior and local-domain resolution **in a separately authorised active-probe stage** (no DNS queries in this collector).
5. Measure total memory, VmRSS and storage. PSS of running RouterForge DNS Router only once intentionally installed and started on DSTest with separate authorisation.
6. If the evidence is missing, report **NOT_VERIFIED**, not PASS.

### Future activation gate (not implemented)

A future DPR2-G/H may implement a transactional DNS Proxy adapter ONLY after an exact dry-run diff and on-device reversible acceptance. It must snapshot original Keenetic config, prevent recursive resolver chains, preserve local names, protect both TCP and UDP and IPv6, use a bounded rollback watchdog, verify restoration, and report `AMBIGUOUS` rather than success if state cannot be proven. No system DNS interception may activate from package postinst or startup.

## Run the hardware collector when explicitly desired

**On chosen DSTest only**, over SSH, stream `scripts/dpr2-f-hardware-readonly.sh` into `sh` via stdin. This is a read-only collector: no package installation, no shell redirection to router paths, no restarts or firewall changes. The collected output contains the device IP/name-server configuration and should be treated as diagnostics, not public telemetry.

## Acceptance

`go test` and `go test -race` must pass, `go vet` and `gofmt` clean. The CI verifier must build separate ARM64/MIPS/MIPSEL IPKs, confirm independence from `routerforge-dns`, absence of automatic installation hooks, and the read-only CLI smoke. Runtime hardware compatibility is not claimed based solely on CI.
