# RouterForge DPR2-E — guarded manual lifecycle, resource evidence, read-only Keenetic discovery

## Exact chain

- Base: `04b3a671dfab4150b3320417d733d789c4f2c5c5` (`feature/dns-router-r7-stage-d`, CI 40557532197 SUCCESS).
- Target: `feature/dns-router-r7-stage-e` created only if absent, single commit, no protected ref mutation.
- Experimental standalone IPK: `routerforge-dns-router_0.14.0~dev.dpr2e_<arch>.ipk`, three architectures. Not released to App Center/Stable/Beta.

## Scope delivered

1. **Guarded manual DNS listener lifecycle:** management-only API is the default. `POST /v1/shadow/control` with action `shadow-control` supports `start`, `stop`, `apply`. `start`/`apply` require a saved exact `If-Match` config revision. The shadow listener is allowed only on loopback and an unprivileged high port. Neither package installation nor config save starts a listener.
2. **Staged apply / in-memory rollback:** running configuration remains unchanged after saving until an explicit `apply`. The engine compiles the candidate prior to touching the live listener. Apply stops the previous shadow listener and starts the new one; on startup failure it attempts to restore the old runtime snapshot. An unverified rollback reports `ambiguous`, not PASS. Persistent user configuration is never overwritten by shadow rollback.
3. **UI:** explicit shadow lifecycle buttons, passive resources and Keenetic discovery. Confirmation before manual start/apply; no new third-party runtime dependencies.
4. **Passive resource snapshot:** Go heap alloc/sys, goroutine count, RSS from `/proc/self/status`, PSS from `/proc/self/smaps_rollup` when available. These metrics are observations, not memory guarantees.
5. **Read-only Keenetic compatibility inventory:** checks only presence of `ndmc`, `/opt`, proc TCP/UDP bind table entries on port 53; no `ndmc` execution, no active DNS probes or firewall modification. Port 53 counts do not establish ownership; actual native DNS Proxy adapter remains **unverified**.
6. **Regression/benchmarks:** race tests, CLI/API mutation tests, stale revision handling, occupied-port startup failure, idle resource endpoint, 10,000-rule evaluation benchmark, cross-architecture IPK metadata checks.

## Safety boundaries

- Port 53 and Keenetic DNS Proxy are not touched; **no DNS override**, no `iptables`, no `ndmc` writes, no background services, no post-install start.
- No arbitrary upstream fallback. Existing DoT/DoH TLS verification remains mandatory.
- `stop` is manual; `apply` is a deliberate runtime change restricted to loopback shadow DNS. When stop exceeds the bounded shutdown window, runtime is explicitly reported ambiguous.
- Core recognizes DNS Router from DPR2-C but no automatic App Center listing or release channel expansion occurs in DPR2-E.
- No hardware acceptance is claimed. CI benchmark on a server is not evidence of MIPS/MIPSEL latency or RSS.

## Verification / next decision

`go test ./modules/dns-router-runtime/...`; `go test -race`; `go vet`; `go test -bench BenchmarkDPR2EEvaluate10K`; three explicit experimental IPK builds. After CI success DPR2-F should implement *read-only hardware probe + safe, explicit Keenetic DNS Proxy integration design* without implicit DNS takeover.
