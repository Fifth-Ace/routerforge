# RouterForge NFQWS Strategy Intelligence — upstream provenance

Base RouterForge integration point:

- Repository: `Fifth-Ace/routerforge`
- Base commit: `2b68231374d5bab08ef445886a5860e9d731663d`
- Workstream: BIG WORK C1 — Candidate Intelligence + Verified Target Memory

Upstream references reviewed for this work:

## Omn1z/nfqws2-keenetic-strategy-selector

- Commit reviewed: `bf4e810ef22ffb6671e97dc411234ba9430909c9`
- Relevant concepts/files:
  - `internal/services/strategy/core/catalog/strategy.go`
  - `internal/services/strategy/core/engine/sandbox.go`
  - `internal/app/run.go`
  - `internal/tools/probe/probe.go`
  - `internal/tools/tlsblob/*`
- Concepts adapted:
  - curated candidate families
  - source-independent candidate representation
  - strategy de-duplication
  - verified-result reuse as a future-run hint
- RouterForge does not vendor the upstream package or copy its runtime architecture.
  Candidate execution remains inside RouterForge's existing reserved-NFQUEUE,
  cleanup-proof and Safe Apply contracts.

## necronicle/z2k

- Commit reviewed: `2d41b8f092d42d4c5af78c974f81a7900b70a17f`
- Relevant concepts/files:
  - `files/lua/z2k-state-persist.lua`
  - `lib/strategies.sh`
  - `z2k-detect/cmd/z2k-detect/main.go`
  - `files/z2k-tcp16-probe.sh`
- Concepts adapted in C1:
  - persistent per-target learning
  - explicit separation between runtime evidence and production mutation
  - strategy-family diversity as input to automatic selection
- RouterForge intentionally does **not** adopt transparent production autocircular
  mutation. Learned evidence only changes candidate ordering/availability; applying
  a strategy still requires RouterForge Recommendation -> Preview -> explicit Safe Apply.

The RouterForge developer reports explicit permission from the upstream developers
to learn from/adapt their source. This file records exact provenance so future
maintenance can distinguish inherited ideas from RouterForge-specific safety design.

Planned follow-up workstreams (not claimed complete by C1):

- C2: multi-protocol bench (HTTP/TCP, QUIC/UDP, generic UDP/voice)
- D: advanced detector, TCP16 escalation, DNS matrix, ClientHello tooling
- E: device activity / trace / optional packet capture / geo-list intelligence
## NFQWS Intelligence R2 — Strategy Registry Foundation

- RouterForge base reviewed: `eac054312891ee2b31e47323039de27a0d88c654`
- z2k reference reviewed: `fca1ed5a452f2554b3dfa1ab18571cee7c505174`
- Omn1z selector reference reviewed: `bf4e810ef22ffb6671e97dc411234ba9430909c9`
- nfqws-zapret-converter reference reviewed: `c37858b8ffead9377f1e27de756c8f5c46c23090`

R2-F0 introduces a RouterForge-native, read-only Strategy Registry contract.
It aggregates existing RouterForge built-ins, Candidate Library entries and verified
Target Memory evidence by technique fingerprint. No production configuration,
runtime lifecycle or Safe Apply behavior changes in this stage.

The registry schema intentionally includes provenance as a first-class field.
Legacy Candidate Library entries do not yet persist an exact upstream commit, so
the API reports that limitation instead of inventing provenance. Later import
workstreams will populate exact source repository/ref metadata at ingestion time.

## NFQWS Intelligence R2-F1 — TCP16 Network Memory

- RouterForge base: `e9639284153dbb2d3409d8e47ea30bad0fb8d23b`
- z2k TCP16 reference reviewed: `necronicle/z2k@fca1ed5a452f2554b3dfa1ab18571cee7c505174`
- Concept reused with permission: persistent network-oriented TCP16 evidence and network-to-SNI memory.
- RouterForge implementation is native: its own JSON schema, `/24` network identity fallback, persistence, API, UI and safety boundary.
- This stage does not copy z2k runtime mutation, scheduler or automatic production configuration behavior.
- R2-F1 records evidence only from RouterForge's existing domain detect path and exposes a read-only API/UI.

## NFQWS Intelligence R2-F2A — ClientHello Lab

- RouterForge base: `668b6433291ea7ecee072333fae590d04fc7ad84`
- Omn1z reference reviewed: `Omn1z/nfqws2-keenetic-strategy-selector@bf4e810ef22ffb6671e97dc411234ba9430909c9`
- Concepts reused with permission: generate a real TLS ClientHello for a chosen SNI, validate the TLS record/handshake shape, and save it as an nfqws2 fake-payload blob.
- RouterForge implementation uses its existing SHA256-guarded Blob Manager and does not restart/reload nfqws2.
- Device/tcpdump capture is intentionally deferred to R2-F2B; this patch contains no tcpdump installation or packet capture.

## NFQWS Intelligence R2-F2B — ClientHello Device Capture

- RouterForge base: `8255f3e5c8ef4cffc4714f62f39fb2b9edccc6ff`
- Omn1z reference: `Omn1z/nfqws2-keenetic-strategy-selector@bf4e810ef22ffb6671e97dc411234ba9430909c9`
- Concept reused with permission: bounded tcpdump capture of a LAN device's outbound TLS ClientHello, pcap extraction, SNI display, and explicit handoff to blob save.
- RouterForge implementation is native and uses the shared `safety.CommandContext` boundary; tcpdump is never installed automatically.
- Capture is bounded to 8 seconds, 64 matching packets, 2 MiB pcap and 24 candidates.
- Capturing does not write nfqws2 config and does not restart/reload production.

## NFQWS Intelligence R2-F3 — Zapret Import V2

- RouterForge base: `8e264fa6aea9c44dd279dbf86dfe37324749fcc1`
- Converter reference reviewed: `whxtelxs/nfqws-zapret-converter@c37858b8ffead9377f1e27de756c8f5c46c23090`
- Existing RouterForge preview-first parser remains authoritative and intentionally does not silently apply converter heuristics.
- R2-F3 adds an atomic Strategy Library handoff: all READY profiles are validated as bench-eligible first, deduplicated by deterministic fingerprint, and then written once with `source=zapret`.
- An invalid item rejects the whole batch; existing identical fingerprints are preserved and reported as existing.
- No active nfqws2 config mutation, reload, restart or automatic apply is performed.

## NFQWS Intelligence R2-F4 — Strategy Registry Enrichment

- RouterForge base: `4be686d5cce9a5cdf9b9833d671aef102437bdc3`
- Zapret converter reference remains pinned to `whxtelxs/nfqws-zapret-converter@c37858b8ffead9377f1e27de756c8f5c46c23090`.
- Registry entries now expose derived bench capabilities from RouterForge's own compiler contract: eligible transports, candidate readiness, desync count and strategy tags.
- Zapret provenance identifies the pinned import-adapter reference without claiming that an imported strategy itself originated from that repository.
- Registry remains read-only and does not mutate Strategy Library, target memory or production nfqws2.

## NFQWS Intelligence R2-F5 — Score Engine V2

- RouterForge base: `ec026c39b4281eb83b64e2af6e8989136abcf117`
- Score Engine V2 is RouterForge-native and read-only.
- Inputs: registry success rate, verified count, memory confidence, target coverage, candidate readiness, failure count and unstable count.
- Maximum score is 100. Components are explicit in API output; failures and unstable evidence subtract bounded penalties.
- The engine does not apply, reorder production rules, or mutate Strategy Library/target memory.
- Registry UI displays the score but keeps the existing registry ordering; score-driven recommendation wiring is deferred to a later gate.

## NFQWS Intelligence R2-F6 — Target Memory V3

- RouterForge base: `5c0397fac08b23e2951c1ac2d7181038378c5980`.
- Storage remains backward-compatible with target-memory document version 1; the API now advertises `feature_version=3`.
- Independent observations are accumulated instead of replacing the previous success/complete rates.
- Historical result-class counters are retained: working, unstable and failed/partial observations.
- Legacy entries are hydrated conservatively from their existing verified count and current result class on first reuse.
- A changed environment fingerprint resets trust/streak/rate evidence before recording the new observation, preventing confidence from leaking across production-config/environment changes.
- Strategy Registry consumes the richer observation counters, so Score Engine V2 receives historical failure/unstable evidence instead of only the latest class.
- No active nfqws2 config, process or rules are changed.

## NFQWS Intelligence R2-F7 — Strategy Insights

- RouterForge base: `0193f41ba5a989a280c48bfc2ec4ca74b562d518`.
- Strategy Insights is a deterministic read-only interpretation layer over Strategy Registry + Score Engine V2.
- States are descriptive evidence states, not automatic decisions: `NOT_READY`, `UNVERIFIED`, `PROVEN`, `PROMISING`, `MIXED`, `DEGRADED`, `WEAK`.
- Every insight carries a concise summary and machine-readable signals such as score, verification count, target coverage, confidence and failure/unstable counts.
- Insights do not alter Registry ordering, progressive selection, Strategy Library, Target Memory, Apply gates or production nfqws2.

## NFQWS Intelligence R2-F8 — NFQWS Jobs via Maintenance scheduler

- RouterForge base: `3804d58ef9fa385c75bc9874dd024dd95cfcdae2`.
- Admin Maintenance owns a persistent NFQWS job scheduler at `/opt/etc/routerforge/nfqws-jobs.json`.
- R2-F8 job kind is `detect-target`: a bounded scheduled call to the existing RouterForge NFQWS `/v1/v2/detect` diagnostic route over the local Unix socket.
- Jobs are disabled until the user explicitly saves/enables one. Minimum interval is 15 minutes; maximum is 24 hours; at most 32 jobs.
- Scheduler waits one full configured interval after Admin startup/configuration instead of immediately probing on service restart.
- Run-now remains explicit and confirmed in the UI.
- The scheduler uses RouterForge's local module authorization header and does not shell out, edit `nfqws2.conf`, restart/reload nfqws2, or alter production firewall rules.
- Existing detect semantics may enrich RouterForge-owned TCP16 intelligence memory; production nfqws2 remains untouched.

## NFQWS Intelligence R2-F9 — Policy Automation Gate

- RouterForge base: `a458298cddd113b94451b7e3031e54f812fa2fd8`.
- This stage adds a read-only maturity gate, not policy automation itself.
- `automation_enabled` is hard-coded `false` and policy `action` is `none`.
- A candidate is gate-ready only when all conditions hold: CandidateReady, `PROVEN` insight, score >= 80, at least 3 verified observations, success rate >= 0.90, STRONG/TRUSTED confidence, zero historical failures, zero unstable observations, and at least one reuse-eligible target.
- Failed conditions are returned as explicit machine-readable reasons.
- The Registry UI surfaces READY/BLOCKED state; it does not change ordering, invoke Smart Apply, create previews, reload/restart nfqws2, or edit production config.
- Actual autonomous policy execution remains deferred behind a separate future gate.

## NFQWS Intelligence R3-F0 — Recommendation Engine V1

- RouterForge base: `d6b35c4eb0e9ae3f505f8ac527022cb2f6807ffa`.
- R3 starts with a read-only historical recommendation layer; it does not change Progressive Selector ordering yet.
- The engine consumes Score Engine V2, Strategy Insights and Policy Automation Gate output.
- Only bench-ready strategies with verified evidence are ranked.
- Ranking is protocol-local and deterministic: insight maturity, score, verified count, then fingerprint.
- `PROVEN`/`PROMISING` top entries may become one advisory primary candidate per protocol.
- Every recommendation explicitly says that live Selector verification is still required before Preview/Safe Apply.
- No active config, process, NFQUEUE production rule, service lifecycle or automation state is changed.

## NFQWS WEB COMPLETE Campaign R1 — upstream integration batch

- RouterForge base: `578c45761563a9a7375307fd5def0f94913c9eee`.
- z2k reference: `necronicle/z2k@fca1ed5a452f2554b3dfa1ab18571cee7c505174`.
- Omn1z selector reference: `Omn1z/nfqws2-keenetic-strategy-selector@bf4e810ef22ffb6671e97dc411234ba9430909c9`.
- Zapret converter reference: `whxtelxs/nfqws-zapret-converter@c37858b8ffead9377f1e27de756c8f5c46c23090`.
- Developer reports explicit upstream permission to reuse/adapt source.
- Campaign R1 deliberately excludes VPN/WARP/AWG/VPS functionality.
- R1 additions:
  - ClientHello generation options and IPv6 pcap parsing adapted from Omn1z `internal/tools/tlsblob/*`;
  - Recommendation Engine output becomes an Auto Pool planning hint only; all candidates still require RouterForge live reserved-NFQUEUE verification;
  - Generic Safe Apply can bind a live-verified non-production candidate only when exactly one existing production profile matches the target; the candidate inherits that source profile's production selection scope before Preview;
  - active config SHA, one-time receipt, deterministic preview, Smart Apply verification and rollback remain RouterForge-native authorities.
