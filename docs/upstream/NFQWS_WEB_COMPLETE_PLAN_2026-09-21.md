# RouterForge NFQWS Manager — Complete Development Plan / Upstream Integration Specification

**Status:** CURRENT development authority for the NFQWS workstream  
**Date:** 2026-09-21  
**Repository:** `Fifth-Ace/routerforge`  
**Working branch:** `dev`  
**Current RouterForge dev SHA:** `578c45761563a9a7375307fd5def0f94913c9eee`  
**Current main SHA:** `cdad293aa464387fcab4db81f97d7da838d106c8`

---

## 0. Purpose of this document

This document is the single detailed engineering plan for finishing RouterForge's NFQWS module to a state where a supported Keenetic + Entware + `nfqws2` installation can be fully operated from the RouterForge Web UI without routine SSH/manual file editing.

It supersedes the old stage ordering where that ordering conflicts with the current repository state, but it does **not** reopen stages already marked CLOSED/PASS. Existing RouterForge safety architecture remains authoritative.

The plan intentionally reuses/adapts mature upstream functionality instead of reimplementing already-solved mechanisms.

### Upstreams reviewed and pinned

1. **RouterForge**
   - Repository: https://github.com/Fifth-Ace/routerforge
   - Working ref: `578c45761563a9a7375307fd5def0f94913c9eee`
   - Immutable tree: https://github.com/Fifth-Ace/routerforge/tree/578c45761563a9a7375307fd5def0f94913c9eee

2. **z2k**
   - Repository: https://github.com/necronicle/z2k
   - Ref: `fca1ed5a452f2554b3dfa1ab18571cee7c505174`
   - Immutable tree: https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174

3. **nfqws2-keenetic-strategy-selector**
   - Repository: https://github.com/Omn1z/nfqws2-keenetic-strategy-selector
   - Ref: `bf4e810ef22ffb6671e97dc411234ba9430909c9`
   - Immutable tree: https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/tree/bf4e810ef22ffb6671e97dc411234ba9430909c9

4. **nfqws-zapret-converter**
   - Repository: https://github.com/whxtelxs/nfqws-zapret-converter
   - Ref: `c37858b8ffead9377f1e27de756c8f5c46c23090`
   - Immutable tree: https://github.com/whxtelxs/nfqws-zapret-converter/tree/c37858b8ffead9377f1e27de756c8f5c46c23090

### Permission / provenance policy

The RouterForge developer reports explicit permission from the upstream developers to adapt/use their implementation. Every substantial adaptation still gets exact provenance in RouterForge:

- repository;
- exact upstream commit;
- exact upstream file(s);
- what was copied/adapted conceptually or structurally;
- RouterForge destination file(s);
- behavioral differences;
- safety-boundary differences.

Existing provenance file:
https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/docs/upstream/NFQWS_STRATEGY_INTELLIGENCE_PROVENANCE.md

Licensing observations at the pinned refs:
- z2k contains an MIT LICENSE:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/LICENSE
- `nfqws-zapret-converter/package.json` declares ISC:
  https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/package.json
- a root `LICENSE` file was not present through the GitHub API at the checked Omn1z ref; project permission/provenance therefore must remain explicitly documented in RouterForge.

---

# 1. Final product definition: NFQWS WEB COMPLETE

The NFQWS workstream is complete only when a user can do the following entirely from RouterForge Web UI on supported hardware:

1. Detect installed `nfqws2`, its version, init script, current process/runtime, dependencies and configuration.
2. Start, stop, restart and reload the installed service with truthful status feedback.
3. Read/edit/validate current and saved NFQWS configuration.
4. Manage domain/IP lists and other auxiliary config files safely.
5. Manage strategy profiles, Candidate Library, strategy sources and strategy provenance.
6. Import strategies from Zapret/Windows strategy sources with preview and deterministic normalization.
7. Generate, validate, capture, store and reuse ClientHello/fake payload blobs.
8. Diagnose a target using DNS/TCP/TLS/HTTP and the supported UDP/QUIC/STUN paths.
9. Observe likely failed destinations/devices automatically and send them to Diagnose/Sandbox/Auto Selector.
10. Distinguish ordinary strategy failure from infrastructure failure and TCP16/volume-cutoff behavior.
11. Run isolated strategy candidates through reserved NFQUEUE without mutating production.
12. Use historical memory/score/insights/recommendations only as hints.
13. Always perform fresh live verification before recommending a production change.
14. Preview the exact candidate config against the exact active config SHA.
15. Apply a verified candidate through the existing Smart Apply transaction:
    backup → atomic write → reload/restart → health verification → rollback on first real failure.
16. Run scheduled diagnostic/revalidation jobs without hidden production mutation.
17. Inspect run history, logs, backups, restore points and maintenance results.
18. Recover cleanly from missing optional dependencies and show the required action.
19. Complete all supported workflows without routine SSH/manual config editing.
20. Preserve RouterForge's explicit no-autonomous-production-mutation policy unless a separate future automation workstream is deliberately opened.

The intended lifecycle is:

`OBSERVE → DIAGNOSE → CLASSIFY → CANDIDATES → SANDBOX → SCORE → REMEMBER → INSIGHTS → RECOMMEND → LIVE VERIFY → PREVIEW → EXPLICIT SAFE APPLY → VERIFY / ROLLBACK`

---

# 2. Explicit non-goals for the NFQWS module

The following upstream functionality is outside this workstream and must not be pulled into the NFQWS module:

- WARP;
- AmneziaWG / WireGuard / VPN provisioning;
- VPS relay/proxy infrastructure;
- Telegram MTProto/SOCKS/WS proxy;
- generic port forwarding;
- ARP spoofing;
- Pi-hole management;
- a second RouterForge DNS server;
- a second generic system monitor;
- a second generic conntrack UI if RouterForge Network Tools can provide the observation surface.

Where upstream code mixes these concerns with NFQWS, only the NFQWS/DPI-relevant logic is adapted.

---

# 3. Existing RouterForge functionality that remains authoritative

These layers are already implemented/closed and are **not rewritten**:

## 3.1 Core NFQWS manager

Current design document:
https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/docs/NFQWS2_MANAGER.md

Core runtime:
https://github.com/Fifth-Ace/routerforge/tree/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime

Frontend:
https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager/frontend/index.html

## 3.2 Existing safety/mutation authority

Keep RouterForge-native:

- exact active config SHA;
- bounded validation;
- safety backup;
- atomic write;
- reload/restart verification;
- rollback;
- one-time receipt/token binding;
- stale-preview rejection;
- no production mutation from intelligence/read-only flows.

Relevant files:

- Smart Apply:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/smart_apply.go
- Smart Apply tests:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/smart_apply_test.go
- AutoTune apply gate:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_autotune_gate.go
- Apply implementation:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_autotune_apply.go
- Preview:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_autotune_preview.go
- Shared atomic safety:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/internal/safety/atomic.go
- Process command boundary:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/internal/safety/command.go

## 3.3 Existing candidate execution authority

Keep RouterForge reserved-NFQUEUE execution, cleanup proof and live candidate comparison.

Relevant files:

- selector:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_selector.go
- candidate pool:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_candidate_pool_v2.go
- progressive selector:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_progressive_selector_v2.go
- progressive selector tests:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_progressive_selector_v2_test.go
- bench lifecycle:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_lifecycle.go
- firewall serialization:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_firewall_serialization_test.go
- transaction:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/bench_transaction.go

## 3.4 Existing intelligence authority

Already closed:
- Strategy Registry;
- TCP16 Network Memory storage foundation;
- ClientHello generate/validate;
- bounded ClientHello capture;
- Zapret Import V2;
- Registry enrichment;
- Score Engine V2;
- Target Memory V3;
- Strategy Insights;
- NFQWS Jobs foundation;
- Policy Automation Gate;
- Recommendation Engine V1.

Relevant files:

- Strategy Registry:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_registry_v1.go
- Target Memory:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_target_memory_v2.go
- TCP16 Memory:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/tcp16_network_memory_v1.go
- Score:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_score_v2.go
- Insights:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_insights_v2.go
- Policy automation gate:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/policy_automation_gate_v2.go
- Recommendation Engine V1:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_recommendations_v2.go
- Recommendation tests:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_recommendations_v2_test.go

---

# 4. Canonical final architecture

The final module is divided into seven authorities.

## 4.1 Observation Authority

Inputs:
- manual target;
- saved list target;
- Observed Target from device/network activity;
- scheduled target job;
- TCP16 curated target;
- imported upstream target set.

Outputs:
- normalized target identity;
- source/provenance;
- timestamp;
- device/source IP where applicable;
- protocol/port;
- failure reason/class;
- destination IP/network identity.

This authority must never mutate production NFQWS.

## 4.2 Diagnostic Authority

Runs controlled probes:
- DNS;
- TCP connect;
- TLS handshake;
- HTTP;
- HTTP body/progress/16K cutoff;
- QUIC where supported;
- STUN/generic UDP where supported.

Outputs:
- infrastructure state;
- target reachability;
- protocol-specific evidence;
- cutoff evidence;
- candidate-required/no-candidate-needed classification.

## 4.3 Candidate Authority

Sources:
- production profiles;
- Target Memory;
- curated RouterForge builtins;
- z2k-derived curated catalog;
- Omn1z-derived catalog/families;
- Candidate Library;
- Zapret import;
- user manual candidate;
- historical Recommendation Engine.

Everything becomes the same RouterForge-native candidate representation with deterministic technique fingerprint and source provenance.

## 4.4 Sandbox Authority

Only RouterForge's isolated NFQUEUE implementation executes candidates.

Historical score cannot mark a candidate as working.
Registry membership cannot mark a candidate as working.
A copied upstream strategy cannot bypass compilation/safety gates.

## 4.5 Evidence Authority

Persists:
- live attempts;
- result class;
- transport;
- target;
- network;
- strategy technique fingerprint;
- success/failure/unstable counters;
- performance metrics;
- cutoff behavior;
- cleanup proof;
- environment fingerprint;
- last verified time.

Feeds:
- Target Memory;
- TCP16 Memory;
- Strategy Registry;
- Score;
- Insights;
- Recommendation Engine.

## 4.6 Planning Authority

Uses historical evidence only to:
- prioritize known-good technique fingerprints;
- preserve family diversity;
- avoid known-bad candidates;
- prefer candidates relevant to current transport/network;
- select a bounded progressive pool.

It never skips:
- baseline;
- live candidate testing;
- cleanup;
- repeated winner verification;
- max-candidate budget.

## 4.7 Mutation Authority

The only production mutation path remains:

`verified winner → compile deterministic candidate config → issue bound receipt → preview → recheck exact active SHA → explicit Apply → backup → atomic write → lifecycle verify → rollback on failure`.

---

# 5. Upstream functionality map

# 5.1 z2k: what we reuse

Repository:
https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174

## A. TCP16 / connection-volume cutoff subsystem

Primary implementation:
- line/network probe script:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-tcp16-probe.sh
- detector implementation:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/tcp16/tcp16.go
- runtime network→SNI logic:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-tcp16.lua
- curated probe targets:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lists/tcp16_targets.txt
- SNI whitelist candidates:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lists/sni_wl_candidates.txt
- network map seed/reference:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lists/tcp16_nets.txt

Reuse:
- separate TCP16 classification from ordinary strategy failure;
- curated line probe;
- per-network identity;
- network-specific working-SNI search;
- batch/parallel search;
- previous-known-working SNI first;
- failed revalidation does not destroy last-known-good state;
- atomic replacement of SNI map;
- freshness/revalidation model.

Do **not** copy:
- self-downloading executables;
- z2k installation/update machinery;
- direct config mutation;
- scheduler shell process.

RouterForge destination:
- new `tcp16_probe_v2.go`;
- new `tcp16_candidates_v2.go`;
- extend `tcp16_network_memory_v1.go`;
- Web TCP16 panel;
- RouterForge Jobs integration.

## B. Runtime observation / likely blocked targets

Implementation:
- blocked monitor:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-blocked-monitor.sh

Reuse:
- TCP RST classification;
- repeated SYN/no-SYNACK classification;
- UDP outbound/no-return classification;
- DNS IP→host correlation idea;
- bounded retention;
- dedupe interval;
- derive watched ports from active NFQWS config.

Do not copy:
- unbounded background tcpdump process as RouterForge's primary architecture;
- flash-heavy state;
- shell AWK runtime if RouterForge can obtain equivalent information from Network Tools/conntrack.

RouterForge destination:
- Network Tools provides flows/conntrack;
- NFQWS adapter converts suspicious flow state into `ObservedTarget`;
- optional short bounded packet observation only when conntrack evidence is insufficient.

## C. Strategy pools / autocircular learning semantics

Implementation:
- strategy definitions/apply logic:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/lib/strategies.sh
- QUIC strategy catalog:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/quic_strats.ini
- persistent state:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-state-persist.lua
- modern Lua core:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-modern-core.lua
- fake/fooling helpers:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-fooling-ext.lua
- QUIC behavior:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-quic-silence.lua
- range randomization:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-range-rand.lua

Reuse:
- mature strategy families as catalog data;
- protocol/family tags;
- prior successful strategy gets earlier priority;
- family diversity;
- fallback/degrade behavior;
- persistent success concept.

Do not copy:
- transparent production autocircular mutation;
- silent runtime selection as RouterForge production authority.

RouterForge maps these semantics into:
`Registry → Target Memory → historical recommendation → progressive planning → fresh live test`.

## D. Detector/classifier ideas

Files:
- main detector:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/cmd/z2k-detect/main.go
- generic prober:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/prober/prober.go
- failure taxonomy:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/prober/failures.go
- classification:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/classify/classify.go
- decision:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/decision/decision.go
- HTTP/TLS specifics:
  https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/prober
- QUIC:
  https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/quicprobe
- voice/STUN:
  https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/voiceprobe

Reuse:
- reason taxonomy;
- distinction between DNS/TCP/TLS/HTTP failures;
- cutoff-specific classification;
- QUIC/UDP evidence interpretation.

Do not replace RouterForge's existing detector wholesale; enrich its reason model.

## E. Scheduler robustness patterns

Files:
- scheduler:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-scheduler.sh
- NFQUEUE self-heal:
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-nfqueue-selfheal.sh

Reuse:
- first-run vs periodic cadence distinction;
- backoff;
- do not erase good state after inconclusive probe;
- no restart storms;
- stale-state awareness;
- explicit health reasoning.

Execution remains RouterForge Maintenance scheduler.

---

# 5.2 Omn1z selector: what we reuse

Repository:
https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/tree/bf4e810ef22ffb6671e97dc411234ba9430909c9

README/capability overview:
https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/README.md

## A. Strategy catalog and sandbox concepts

- strategy catalog:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/catalog/strategy.go
- sandbox:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/engine/sandbox.go
- cleanup:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/engine/cleanup.go
- Linux cleanup:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/engine/cleanup_linux.go
- run orchestration:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/app/run.go

Reuse:
- candidate families/catalog entries;
- useful metric presentation;
- multi-worker concept;
- any missing cleanup edge cases.

Keep RouterForge sandbox execution authority.

## B. Devices / connections / observation

- monitor:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/monitor/monitor.go
- pcap:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/monitor/pcap.go
- trace:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/monitor/trace.go
- conntrack parser:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/netmon/conntrack.go
- devices:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/netmon/devices.go
- queue stats:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/netmon/queue.go
- proc/system observation:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/tree/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/netmon

Frontend references:
- Connections:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/connections/Connections.tsx
- Devices:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/devices/Devices.tsx

Reuse:
- data model for device→flow→failed target;
- conntrack parsing patterns;
- trace UX;
- one-click handoff from observed flow to strategy test.

RouterForge must expose this through existing Network Tools / Flow Explorer where possible.

Current RouterForge Network Tools:
- flow attribution:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/network-tools/runtime/flow_attribution.go
- flow explorer:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/network-tools/runtime/flow_explorer.go
- route inspector:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/network-tools/runtime/route_inspector.go

## C. Probe

- probe implementation:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/probe/probe.go
- control Linux:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/probe/control_linux.go

Use as comparison/reference for:
- connection success;
- response completeness;
- performance metrics;
- target status.

## D. ClientHello / blob pipeline

- generator:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/tlsblob/generate.go
- pcap parser:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/tlsblob/pcap.go
- one-frame SNI sniffer:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/tlsblob/sniff.go
- blob capture service:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/blobs/capture.go
- blob service:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/blobs/blobs.go
- trash:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/blobs/trash.go
- frontend:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/blobs/Blobs.tsx

RouterForge already implements generate/validate/capture:
- generator/validator:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/clienthello_lab_v1.go
- capture:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/clienthello_capture_v1.go
- blob manager:
  https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/blobs.go

Remaining parity targets:
- IPv6 packet parsing in capture;
- ALPN options;
- minimum TLS version option;
- better captured-candidate UX;
- optional trash/restore semantics for user blobs;
- clear source/provenance.

## E. GeoSite / GeoIP / source lists

- Geo implementation:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/geo/geo.go
- lookup:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/geo/lookup.go
- app/API:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/app/app_geo.go
- update:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/app/app_geo_update.go
- frontend:
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/geo/Geo.tsx

Adapt into RouterForge list/source architecture rather than adding a second storage subsystem.

---

# 5.3 whxtelxs converter: what we reuse

Repository:
https://github.com/whxtelxs/nfqws-zapret-converter/tree/c37858b8ffead9377f1e27de756c8f5c46c23090

Primary reference:
https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/analyze.js

README:
https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/README.md

Sample strategy corpus:
https://github.com/whxtelxs/nfqws-zapret-converter/tree/c37858b8ffead9377f1e27de756c8f5c46c23090/strats

Reuse as a parser/compatibility oracle for:
- log format 1;
- log format 2;
- HTTP / TLS1.2 / TLS1.3 result extraction;
- success-rate calculation;
- Windows `.bat` multiline `^`;
- `winws.exe` / `nfqws` command extraction;
- `%~dp0`, `%BIN%`, `%LISTS%` normalization;
- `--wf-tcp`, `--wf-udp` removal;
- `--new` splitting;
- hostlist/ipset path conversion;
- known blob-name normalization;
- production of normalized NFQWS argument blocks.

Do **not**:
- run Node on router;
- copy interactive CLI;
- treat converter's ranking as RouterForge production recommendation;
- silently discard unsupported tokens.

RouterForge authoritative import path:
https://github.com/Fifth-Ace/routerforge/blob/578c45761563a9a7375307fd5def0f94913c9eee/modules/nfqws-manager-runtime/strategy_import_v2.go

Required improvement:
turn upstream examples into deterministic RouterForge fixtures and report:
`READY / READY_WITH_WARNINGS / UNSUPPORTED`.

---

# 6. Data model additions required to finish the module

## 6.1 ObservedTarget

New normalized model:

```text
ObservedTarget
  id
  hostname
  destination_ip
  destination_port
  transport
  protocol_hint
  source_device_ip
  source_device_name
  observation_source
  failure_class
  failure_detail
  first_seen
  last_seen
  observation_count
  ignored
```

`observation_source` examples:
- `manual`
- `network-tools`
- `conntrack`
- `blocked-monitor`
- `dns-correlation`
- `scheduled`
- `tcp16-probe`

No production config is changed by storing an observation.

## 6.2 TCP16 evidence

Extend existing entry to preserve:

```text
network_key
CIDR
ASN/provider when known
destination samples
cutoff_detected
cutoff_count
clear_count
working_sni
working_sni_source
working_sni_verified_at
last_probe_at
last_probe_result
last_successful_map_at
candidate_attempts
confidence
```

Key rule:
an inconclusive or infrastructure-failed probe **must not clear** an existing working SNI.

## 6.3 Candidate provenance

Every candidate needs:

```text
source_kind
source_repository
source_ref
source_file
source_record_id
source_strategy_name
normalized_args
technique_fingerprint
transport_capabilities
required_blobs
required_lists
required_lua
```

## 6.4 Evidence reason codes

Canonical reason families:

Infrastructure:
- DNS_FAILURE
- ROUTE_FAILURE
- CONNECT_FAILURE
- LOCAL_DEPENDENCY_MISSING
- NFQUEUE_SETUP_FAILURE
- CLEANUP_FAILURE
- CONFIG_CHANGED

Target/protocol:
- TCP_RST
- TCP_NO_SYNACK
- TLS_TIMEOUT
- TLS_ALERT
- HTTP_BLOCKED
- HTTP_INCOMPLETE
- CUTOFF_16K
- QUIC_NO_RESPONSE
- UDP_NO_RESPONSE

Candidate:
- COMPILE_UNSUPPORTED
- ATTEMPT_FAILED
- PARTIAL
- UNSTABLE
- WORKING

This makes UI and planner decisions deterministic and explainable.

---

# 7. Development stages

The stages below are the authoritative order for completing the module.

---

# STAGE U0 — Upstream Baseline & Provenance Lock

## Goal

Before new code, pin exact upstream sources and create a machine-readable inventory so later work never drifts to "latest" unintentionally.

## Work

1. Update:
   `docs/upstream/NFQWS_STRATEGY_INTELLIGENCE_PROVENANCE.md`
2. Add a current upstream manifest, for example:
   `docs/upstream/nfqws-upstreams-2026-09-21.json`
3. Record:
   - repo;
   - commit;
   - relevant file;
   - intended RouterForge stage;
   - reuse mode: `concept`, `algorithm`, `data`, `adapted-code`;
   - local destination.
4. Document explicit permission.
5. Add CI test that every referenced RouterForge destination exists and every provenance record is syntactically valid.
6. No runtime behavior change.

## Acceptance

- no production mutation;
- exact upstream refs fixed;
- CI PASS;
- `main` untouched;
- dev/tag exact SHA.

---

# STAGE U1 — Strategy Corpus Integration

## Goal

Stop relying on a tiny home-grown candidate set. Build a substantial curated Registry-backed candidate corpus from z2k and Omn1z.

## Upstream sources

z2k:
- `lib/strategies.sh`
- `quic_strats.ini`
- relevant Lua strategy helpers

Omn1z:
- `internal/services/strategy/core/catalog/strategy.go`

## Implementation

1. Introduce a static/import-time catalog representation in RouterForge.
2. Import only strategy families compatible with installed `nfqws2` capabilities.
3. Normalize every candidate through the existing RouterForge compiler.
4. Compute deterministic technique fingerprint after normalization.
5. Deduplicate cross-upstream duplicates.
6. Keep source identity even when technique fingerprint collides:
   Registry may show multiple provenance sources attached to one canonical technique.
7. Reject candidates requiring unavailable Lua/blob/list resources.
8. Do not install missing arbitrary resources automatically.
9. Catalog refresh/version is tied to RouterForge module release, not fetched from upstream at runtime unless a later explicit source-refresh feature is added.

## UI

Strategy Registry adds:
- source repository;
- upstream strategy name;
- protocol/family;
- required resources;
- candidate ready/not ready;
- compile reason;
- historical evidence;
- score/insight.

## Tests

- normalization fixtures;
- duplicate fingerprint fixtures;
- missing-resource fixtures;
- TCP candidate compile;
- QUIC candidate compile;
- no source bypasses safety compiler;
- max candidate limits unchanged.

## Production mutation

None.

---

# STAGE U2 — TCP16 Probe V2: real network-specific cutoff intelligence

## Goal

Turn current passive TCP16 memory into the full useful workflow already demonstrated by z2k.

## Upstream sources

Primary:
- `files/z2k-tcp16-probe.sh`
- `z2k-detect/internal/tcp16/tcp16.go`
- `files/lua/z2k-tcp16.lua`
- `files/lists/tcp16_targets.txt`
- `files/lists/sni_wl_candidates.txt`

## Architecture

`curated targets → baseline line probe → affected networks → SNI candidate scan per network → verified network→SNI memory`

## RouterForge implementation

1. `tcp16_probe_v2.go`
   - bounded parallel probe;
   - explicit maximum targets;
   - explicit request timeout;
   - total run deadline;
   - metrics necessary to distinguish cutoff from ordinary network failure.
2. `tcp16_candidates_v2.go`
   - candidate SNI list;
   - previous known good SNI moved to front;
   - bounded batch test.
3. Persist results into existing TCP16 memory.
4. Atomic update:
   - build new verified map in memory/temp structure;
   - commit only if probe completed sufficiently;
   - preserve previous known-good entries on inconclusive run.
5. Confidence:
   - known SNI + repeated cutoff verification => high;
   - single observation => observed;
   - repeated clear => clear.
6. Never mutate production config in this stage.
7. The recommendation layer may later use TCP16 result as a diagnostic hint:
   "ordinary strategy rotation is unlikely to solve this; TCP16-specific technique/resource is needed."

## Scheduler integration

Use RouterForge NFQWS Jobs:
- manual Run Now;
- daily/periodic revalidation;
- minimum interval protections;
- no startup storm;
- no concurrent duplicate TCP16 job;
- retain last-known-good on failed/incomplete run.

## UI

Dedicated TCP16 section:
- last probe;
- age;
- number of networks checked;
- networks with suspected cutoff;
- networks with verified working SNI;
- last successful SNI scan;
- run status/live progress;
- "Run probe";
- "Revalidate";
- detailed table.

## Acceptance

Hardware:
- production config SHA unchanged;
- production PID unchanged unless the underlying diagnostic explicitly requires no restart;
- no production NFQUEUE rule change;
- temp artifacts cleaned;
- existing known-good state preserved after forced infrastructure failure.

---

# STAGE U3 — Observed Targets / Device Activity

## Goal

User should not need to know which domain is broken. RouterForge should surface likely failed destinations from real LAN activity.

## Upstream sources

Omn1z:
- `internal/tools/netmon/conntrack.go`
- `internal/tools/netmon/devices.go`
- `internal/services/monitor/monitor.go`
- `internal/services/monitor/trace.go`
- `frontend/src/features/connections/Connections.tsx`
- `frontend/src/features/devices/Devices.tsx`

z2k:
- `files/z2k-blocked-monitor.sh`

RouterForge integration:
- `modules/network-tools/runtime/flow_explorer.go`
- `modules/network-tools/runtime/flow_attribution.go`

## Architecture

Do **not** create another generic monitoring subsystem.

Preferred flow:

`RouterForge Network Tools flow data → NFQWS observation adapter → failure classifier → ObservedTarget store`

## Classifiers

At minimum:
- TCP SYN retries with no SYN/ACK;
- TCP RST;
- repeated short failed connects;
- UDP egress with no matching return within bounded window;
- DNS name correlation where available;
- destination port must be relevant to active NFQWS policy or explicit user interest.

## False-positive controls

- local/private destinations excluded by default;
- ignored-target list;
- minimum event count before showing noisy candidate;
- bounded retention;
- dedupe;
- no "blocked" label unless evidence supports it: UI should say "suspected failure" / "candidate for diagnostics".

## UI

New "Observed" panel under NFQWS:
- Device
- Target / destination
- Protocol / port
- failure reason
- count
- last seen
- actions:
  - Diagnose
  - Sandbox
  - Auto Select
  - Add to List
  - Ignore

## Acceptance

- no production mutation;
- no permanent tcpdump daemon required for normal operation;
- bounded memory/storage;
- correct dedupe;
- one click carries target into existing diagnostics/selector.

---

# STAGE U4 — Diagnostic Classifier Parity

## Goal

Make RouterForge's Diagnose result as useful as the mature z2k detector while preserving RouterForge's implementation.

## Upstream references

- z2k detector main
- `internal/prober/*`
- `internal/classify/*`
- `internal/decision/*`
- QUIC and voice probe packages

## Work

1. Audit RouterForge current detector against z2k failure taxonomy.
2. Add missing result reasons rather than replacing working probes.
3. Separate:
   - infrastructure unavailable;
   - target genuinely unreachable;
   - DPI-like TLS/HTTP failure;
   - 16K cutoff;
   - strategy not needed.
4. Ensure the same reason codes feed:
   - UI;
   - run history;
   - Target Memory;
   - Score/Insights;
   - Recommendation/Planner.
5. Improve user-facing next action:
   - DNS failed → inspect DNS;
   - TCP failed → route/connectivity;
   - TLS/HTTP DPI candidate → run selector;
   - TCP16 suspected → TCP16 probe;
   - baseline works → no strategy needed.

## Production mutation

None.

---

# STAGE U5 — ClientHello / Blob Parity

## Goal

Finish the ClientHello feature rather than rebuild it.

## Existing RouterForge

- generation/validation;
- bounded capture;
- blob storage.

## Upstream Omn1z parity targets

From `internal/tools/tlsblob/*` and `internal/services/blobs/*`:

1. IPv6 pcap parsing.
2. Ethernet + Linux SLL + RAW link types.
3. configurable ALPN;
4. configurable minimum TLS version;
5. capture result sorting by size;
6. clear validity/detail;
7. optional trash/restore for user blobs;
8. better device capture flow.

## Safety

- tcpdump never installed silently;
- bounded seconds/packets/bytes;
- no config mutation when capturing/generating;
- blob save is explicit;
- name/path validation;
- SHA shown in UI.

## Acceptance

- generated TLS12/TLS13-compatible variants valid;
- IPv4 + IPv6 capture fixture tests;
- fragmented/truncated pcap rejected or ignored safely;
- no path traversal;
- no production restart.

---

# STAGE U6 — Zapret Import Parser Parity

## Goal

Lock RouterForge import compatibility against the existing converter instead of growing ad-hoc regexes.

## Reference

`whxtelxs/nfqws-zapret-converter/analyze.js`

## Work

Create fixtures for:

### Logs
- Format 1 `Config: ... (Type:...)`;
- Format 2 `[N/M] strategy`;
- HTTP;
- TLS1.2;
- TLS1.3;
- sizes KB/MB/bytes;
- OK/FAIL/LIKELY_BLOCKED;
- partial/incomplete log.

### Batch strategies
- single-line winws;
- multiline caret;
- quoted paths;
- `%~dp0`;
- `%BIN%`;
- `%LISTS%`;
- `--new`;
- `--wf-tcp`;
- `--wf-udp`;
- hostlist;
- hostlist-exclude;
- ipset;
- ipset-exclude;
- known fake blob filenames.

## Required RouterForge behavior

Never silently drop an unknown argument.

Every imported record:
- `READY`;
- `READY_WITH_WARNINGS`;
- `UNSUPPORTED`.

Preview shows:
- input;
- normalized output;
- rewrites;
- dropped/non-runtime wrapper flags;
- unresolved resources;
- final candidate fingerprint.

Only READY/explicitly accepted READY_WITH_WARNINGS can enter Candidate Library.

No direct production Apply from import screen.

---

# STAGE U7 — GeoSite / GeoIP / List Intelligence

## Goal

Finish list sourcing/creation so users can build NFQWS policy sets without shell scripts.

## Upstream reference

Omn1z:
- `internal/tools/geo/geo.go`
- `internal/tools/geo/lookup.go`
- `internal/app/app_geo.go`
- `internal/app/app_geo_update.go`
- `frontend/src/features/geo/Geo.tsx`

z2k:
- `files/z2k-update-lists.sh`
- list corpora only when licensing/provenance is clear.

## RouterForge integration

Use existing:
- `list_sources.go`
- upstream resolver/source manager;
- current list CRUD;
- existing backup/safety.

## Features

- upload GeoSite/GeoIP asset;
- inspect metadata/categories;
- extract selected category;
- preview count;
- dedupe;
- save into NFQWS list;
- optional update source metadata;
- source provenance;
- no automatic production config edit merely because list changed unless the user chooses an existing safe list-save/reload workflow.

## Autohostlist

Expose installed nfqws2 autohostlist functionality as config/UI if supported by installed version. Do not write a new autohostlist engine.

---

# STAGE U8 — Recommendation-aware Progressive Planning (revised R3-F1)

## Goal

Use historical intelligence and mature upstream prior-success semantics to improve candidate order without replacing live verification.

## Inputs

- Recommendation Engine V1;
- Target Memory V3;
- TCP16 diagnostics;
- Strategy Registry;
- curated upstream catalog;
- production profiles;
- Candidate Library;
- manual candidates.

## Rules

1. Baseline always first.
2. Historical recommendation is a hint only.
3. Recommendation compatible with current transport only.
4. If recommendation fingerprint already exists:
   - promote that existing candidate;
   - preserve original source identity.
5. If recommendation corresponds to a Registry/Library candidate whose args compile:
   - it may be admitted through the existing compiler.
6. Never exceed:
   - mode MaxCandidates;
   - stage budgets unless contract is deliberately changed.
7. Dedup by technique fingerprint remains deterministic.
8. Preserve source/family diversity.
9. Known-bad/unstable historical evidence may lower priority but cannot alone declare a candidate failed.
10. Live result ranking remains the selector's result comparator.
11. VERIFY_TOP / repeated stability remains mandatory.
12. Historical score never creates Apply eligibility.

## Response metadata

Expose:
- recommendation snapshot/version;
- recommendation compatible count;
- promoted count;
- admitted-from-registry count;
- source;
- fingerprint;
- historical reason;
- historical score/insight;
- whether candidate was already in pool;
- original stage and effective stage/order.

## UI

Auto Selector clearly shows:
- Historical hint;
- why it was promoted;
- source;
- then live outcome separately.

Do not blur "historically promising" and "live working."

---

# STAGE U9 — Generic Candidate Safe Apply (R3-F2)

## Goal

Any live-verified winner that can be compiled safely should be eligible for Preview/explicit Apply regardless of candidate source.

## Sources allowed

- production;
- Target Memory;
- builtin;
- z2k catalog;
- Omn1z catalog;
- Candidate Library;
- Zapret import;
- manual/custom.

Source itself must not be a blocker.

## Preconditions

- winner is live-verified;
- cleanup proven;
- candidate compiler succeeds;
- required resources exist;
- exact active config SHA still matches;
- no infrastructure failure;
- no stale session.

## Receipt

One-time short-lived receipt binds:
- active config SHA;
- target;
- transport;
- winner fingerprint;
- exact args;
- required resources;
- live verification result;
- session ID;
- issued time/expiry.

Never store receipt in localStorage/DOM as durable state.

## Preview

Server rebuilds candidate config deterministically and returns:
- active SHA;
- candidate SHA;
- exact diff/config;
- changed bool;
- warnings;
- required reload/restart method.

## Apply

Re-check:
- receipt;
- active SHA;
- candidate SHA;
- dependencies.

Then existing Smart Apply:
backup → atomic write → reload/restart → status/lifecycle verify → rollback on failure.

## UI after apply

Refresh:
- status;
- current config;
- run history;
- backups;
- strategy memory where appropriate.

---

# STAGE U10 — End-to-End Automatic Setup UX (R3-F3)

## Goal

One user flow from "something does not work" to safe verified production configuration.

## Workflow

### Entry A: manual target
User types target.

### Entry B: observed target
User clicks a suspicious device/connection.

### Entry C: list target
User selects list/domain.

All enter:

1. Inspect target.
2. Baseline diagnosis.
3. TCP16 branch if indicated.
4. Build bounded candidate plan.
5. Show historical hints.
6. Live isolated candidate tests.
7. Re-verify winner.
8. Explain why winner was chosen.
9. Preview exact config.
10. User explicitly applies.
11. Verify production.
12. Done or rollback.

## UI states must distinguish

- dependency missing;
- DNS failure;
- routing failure;
- target baseline already healthy;
- no candidates compile;
- no candidate works;
- cleanup failure;
- config changed while testing;
- winner verified;
- preview stale;
- apply rollback;
- apply success.

No generic "failed" blob.

---

# STAGE U11 — Jobs / Maintenance / Robustness

## Goal

Make the module self-maintaining without hidden production mutation.

## Existing RouterForge authority

Maintenance scheduler + `/opt/etc/routerforge/nfqws-jobs.json`.

## Job kinds to support

- `detect-target`;
- `tcp16-revalidate`;
- `list-refresh` if list source supports it;
- optional `strategy-health-recheck` for explicit targets;
- backup cleanup.

## Robustness concepts borrowed from z2k

- no immediate repeated job storm after service restart;
- first-run vs periodic cadence;
- stale-state check;
- last-known-good preservation;
- bounded retries;
- single-flight per job kind/key;
- explicit failed/inconclusive distinction.

## NFQUEUE health

Adapt z2k self-heal *diagnostics*, but production mutations must remain gated.

Allowed automatically:
- observe;
- report stale/missing queue;
- flag unhealthy state.

Any action that edits rules/restarts production must go through an explicit RouterForge maintenance action or a separately approved policy.

---

# STAGE U12 — NFQWS Web Completeness Audit

This is a full browser/API audit, not a quick visual check.

## Overview / Runtime

Verify:
- installed;
- version;
- running;
- PID;
- config path;
- init path;
- dependencies;
- actual current status.

## Configuration

Verify:
- setup editor round-trip;
- raw editor;
- validation;
- diff;
- stale SHA protection;
- reload/restart truthful result.

## Lists

Verify:
- create;
- edit;
- delete;
- import;
- dedupe;
- source;
- preview;
- safety;
- list reload behavior.

## Strategies

Verify:
- production inventory;
- builtins;
- upstream catalog;
- Candidate Library;
- Registry;
- provenance;
- compile eligibility;
- strategy resources.

## Auto Selector

Verify:
- baseline;
- candidate pool;
- recommendation hints;
- live progress;
- cancellation;
- cleanup;
- repeated verification;
- no historical auto-win;
- generic preview/apply.

## Zapret Import

Verify:
- both log formats;
- batch parse;
- preview;
- warnings;
- atomic library import;
- no direct apply.

## Sandbox/Test Lab

Verify:
- TCP;
- HTTP/TLS;
- QUIC;
- STUN/generic UDP supported paths;
- resource selection;
- NFQUEUE cleanup;
- performance metrics.

## TCP16

Verify:
- memory;
- real line probe;
- SNI scan;
- revalidation;
- last-known-good preservation;
- scheduler;
- UI.

## Observed Targets

Verify:
- device;
- flow;
- reason;
- dedupe;
- ignore;
- handoff to Diagnose/Selector.

## Blobs/ClientHello

Verify:
- upload;
- generate;
- validate;
- capture IPv4;
- capture IPv6;
- save;
- delete/trash/restore if implemented;
- choose in strategy.

## Geo/list sourcing

Verify:
- asset;
- category;
- preview;
- import;
- provenance;
- update.

## Maintenance

Verify:
- Jobs;
- Run Now;
- history;
- logs;
- backup list;
- restore;
- errors.

## UX

Verify:
- desktop;
- mobile/narrow layout;
- no dead buttons;
- no fake actions;
- no silent 404;
- actionable error messages;
- refreshing state after mutation;
- no unsafe token persistence.

---

# STAGE U13 — Final hardware acceptance

Hardware authority:
supported Keenetic/Entware target, ARM64 first.

## Required pre-snapshot

Record:
- RouterForge core version/SHA;
- NFQWS Manager package version/digest;
- nfqws2 version;
- production PID;
- production config SHA;
- relevant firewall/NFQUEUE state;
- jobs config;
- memory file hashes if test touches intelligence.

## Acceptance suites

### Read-only/intelligence suite
Must prove:
- prod config SHA unchanged;
- prod PID unchanged;
- prod NFQUEUE unchanged.

### Sandbox suite
Must prove:
- reserved queue only;
- actual packets traverse test queue;
- candidate process exists only during test;
- rules/process/temp files removed after every attempt;
- cleanup proof survives cancellation/error.

### Safe Apply suite
Use an intentionally controlled candidate change:
- preview exact;
- active SHA check;
- backup created;
- candidate applied;
- lifecycle verified;
- expected config SHA changes exactly once.

### Rollback suite
Inject a controlled lifecycle failure:
- apply fails;
- rollback restores previous config;
- previous service state returns;
- final SHA equals original.

### TCP16 suite
- run bounded probe;
- create/update evidence;
- failed/inconclusive rerun does not erase last-known-good;
- no production mutation.

### Observed Target suite
- generate a known failed connection;
- target appears;
- Diagnose handoff works;
- ignore/dedupe works.

## Final declaration

Only after all sections pass:

`NFQWS_WEB_COMPLETE: PASS`

---

# 8. Tests required for every new stage

Every stage must include:

1. pure unit tests;
2. parser/fixture tests;
3. negative/error tests;
4. path/safety tests;
5. API contract tests;
6. mutation-boundary tests proving read-only stages do not mutate production;
7. frontend contract/browser tests where UI changes;
8. existing full CI;
9. exact SHA verification after push.

No stage is closed because "it compiles."

---

# 9. RouterForge development execution protocol

For every non-trivial stage:

1. Verify live GitHub remote truth.
2. Verify local branch/head/tree.
3. Exact base SHA gate.
4. No `git worktree`.
5. No `git clean`.
6. No `git reset --hard`.
7. No `git add .`.
8. Disable repo-local auto GC/maintenance before commit.
9. Deterministic changed-file allowlist.
10. `git diff --check`.
11. Targeted tests represented in GitHub Actions Linux.
12. One deterministic ZIP.
13. One short Windows PowerShell 5.1 launcher.
14. Launcher:
    - exact ZIP SHA256;
    - unpack to `%TEMP%`;
    - call `run.cmd -RepoPath ...`;
    - capture exit code;
    - remove temp directory on success;
    - print `<STAGE>_COMPLETE`.
15. Internal runner:
    `PRECHECK → GATES → ACTION → LIVE → VERIFY → RESULT`.
16. After push find exact CI run by pushed SHA.
17. `gh run watch <exact-run-id> --exit-status`.
18. First real RED: stop and print diagnostics; no blind rerun.
19. Success verification:
    - CI head SHA exact;
    - `dev` exact;
    - `routerforge-dev` exact if dev package published;
    - main unchanged;
    - expected assets/digests;
    - local tracked tree clean.
20. After commit: fix-forward only.

Windows PC is not build/test authority and no Go/Node/npm toolchain is installed for RouterForge development.

---

# 10. Stage dependency graph

```text
U0 Provenance
 |
 +--> U1 Strategy Corpus -----------------------------+
 |                                                    |
 +--> U2 TCP16 Probe ---------------------------------|
 |                                                    |
 +--> U3 Observed Targets --> U4 Diagnostics ---------|
 |                                                    |
 +--> U5 ClientHello Parity --------------------------|
 |                                                    |
 +--> U6 Zapret Parser Parity ------------------------|
 |                                                    |
 +--> U7 Geo/List Intelligence -----------------------|
                                                      v
                                      U8 Recommendation-aware Planner
                                                      |
                                                      v
                                      U9 Generic Candidate Safe Apply
                                                      |
                                                      v
                                      U10 End-to-End Auto Setup UX
                                                      |
                                                      v
                                      U11 Jobs / Robustness
                                                      |
                                                      v
                                      U12 Web Completeness Audit
                                                      |
                                                      v
                                      U13 Hardware Acceptance
                                                      |
                                                      v
                                      NFQWS_WEB_COMPLETE
```

U1-U7 can sometimes be developed independently, but U8 should not be considered final until the candidate/evidence sources it plans over are mature.

---

# 11. Definition of DONE per stage

A stage is DONE only when all are true:

- exact code merged to `dev`;
- CI exact SHA PASS;
- required unit/API/frontend tests PASS;
- current provenance updated;
- no unexplained dirty tree;
- no regression in closed stages;
- mutation scope matches stage contract;
- Web UI has no dead path for the new feature;
- hardware acceptance completed immediately when the stage changes hardware/runtime behavior, or explicitly deferred to the named integration acceptance stage;
- handoff/current progress document updated.

---

# 12. Implementation decisions we should not reopen without evidence

1. **RouterForge Safe Apply stays authoritative.**
2. **Historical recommendation never replaces live verification.**
3. **No transparent autocircular production mutation.**
4. **No runtime Node dependency.**
5. **No second generic monitoring subsystem inside NFQWS.**
6. **No second generic DNS server inside NFQWS.**
7. **No VPN/WARP/AWG/VPS work in this module.**
8. **No automatic install of tcpdump or arbitrary strategy dependencies.**
9. **No production config mutation from Registry/Score/Insights/TCP16 observation/Observed Targets.**
10. **No stale preview apply.**
11. **No Apply eligibility based on source name.**
12. **All upstream adaptations are pinned and provenance-recorded.**

---

# 13. Immediate next work from current SHA

Current `dev` is R3-F0 Recommendation Engine V1 at:

`578c45761563a9a7375307fd5def0f94913c9eee`

The old in-progress R3-F1 should be re-scoped as **U8**, not blindly continued first.

Immediate execution order:

1. **U0** — current provenance/manifest lock.
2. **U1** — curated upstream Strategy Corpus.
3. **U2** — TCP16 real probe + per-network SNI selection.
4. **U3** — Observed Targets integrated with Network Tools.
5. **U4** — diagnostic reason parity.
6. **U5** — ClientHello IPv6/options parity.
7. **U6** — Zapret parser parity fixtures.
8. **U7** — Geo/List source completeness.
9. **U8** — corrected recommendation-aware progressive planning.
10. **U9** — generic verified candidate Safe Apply.
11. **U10** — complete one-flow Web setup.
12. **U11** — scheduled maintenance/robustness.
13. **U12** — full Web audit.
14. **U13** — final hardware acceptance.

---

# 14. Upstream file appendix — exact immutable links

## z2k

- README  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/README.md
- Architecture  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/ARCHITECTURE.md
- TCP16 probe  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-tcp16-probe.sh
- TCP16 detector  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/tcp16/tcp16.go
- TCP16 Lua  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-tcp16.lua
- TCP16 targets  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lists/tcp16_targets.txt
- SNI candidates  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lists/sni_wl_candidates.txt
- TCP16 networks  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lists/tcp16_nets.txt
- Blocked monitor  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-blocked-monitor.sh
- Scheduler  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-scheduler.sh
- NFQUEUE self-heal  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/z2k-nfqueue-selfheal.sh
- Strategy manager/catalog logic  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/lib/strategies.sh
- QUIC strategies  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/quic_strats.ini
- State persistence  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-state-persist.lua
- Modern core  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-modern-core.lua
- Fooling extension  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-fooling-ext.lua
- QUIC silence  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-quic-silence.lua
- Range randomizer  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/files/lua/z2k-range-rand.lua
- Detector main  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/cmd/z2k-detect/main.go
- Generic prober  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/prober/prober.go
- Failure taxonomy  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/prober/failures.go
- Classifier  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/classify/classify.go
- Decision engine  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/decision/decision.go
- QUIC probe package  
  https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/quicprobe
- Voice/STUN probe package  
  https://github.com/necronicle/z2k/tree/fca1ed5a452f2554b3dfa1ab18571cee7c505174/z2k-detect/internal/voiceprobe
- Web strategy-pick page  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/webpanel/www/js/pages/strategy-pick.js
- Web strategies page  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/webpanel/www/js/pages/strategies.js
- Web diagnostics page  
  https://github.com/necronicle/z2k/blob/fca1ed5a452f2554b3dfa1ab18571cee7c505174/webpanel/www/js/pages/diag.js

## Omn1z selector

- README  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/README.md
- Strategy catalog  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/catalog/strategy.go
- Sandbox  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/engine/sandbox.go
- Cleanup  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/strategy/core/engine/cleanup.go
- Run orchestration  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/app/run.go
- Monitor  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/monitor/monitor.go
- Trace  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/monitor/trace.go
- PCAP  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/monitor/pcap.go
- Conntrack  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/netmon/conntrack.go
- Devices  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/netmon/devices.go
- Probe  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/probe/probe.go
- ClientHello generate  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/tlsblob/generate.go
- ClientHello pcap parser  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/tlsblob/pcap.go
- ClientHello SNI sniffer  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/tlsblob/sniff.go
- Blob capture  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/blobs/capture.go
- Blob service  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/blobs/blobs.go
- Blob trash  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/blobs/trash.go
- Geo  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/geo/geo.go
- Geo lookup  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/tools/geo/lookup.go
- Geo API  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/app/app_geo.go
- NFQWS2 control  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/internal/services/nfqws2/nfqws2ctl.go
- NFQWS2 frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/nfqws2/Nfqws2.tsx
- Config pane  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/nfqws2/ConfigPane.tsx
- Strategies frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/strategies/Strategies.tsx
- Connections frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/connections/Connections.tsx
- Devices frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/devices/Devices.tsx
- Blobs frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/blobs/Blobs.tsx
- Runs frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/runs/Runs.tsx
- Logs frontend  
  https://github.com/Omn1z/nfqws2-keenetic-strategy-selector/blob/bf4e810ef22ffb6671e97dc411234ba9430909c9/frontend/src/features/logs/Logs.tsx

## nfqws-zapret-converter

- README  
  https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/README.md
- analyzer/converter  
  https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/analyze.js
- sample log  
  https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/log.txt
- sample strategies  
  https://github.com/whxtelxs/nfqws-zapret-converter/tree/c37858b8ffead9377f1e27de756c8f5c46c23090/strats
- package metadata  
  https://github.com/whxtelxs/nfqws-zapret-converter/blob/c37858b8ffead9377f1e27de756c8f5c46c23090/package.json

---

# 15. Final completion checklist

The workstream is not closed until all are PASS:

```text
NFQWS_RUNTIME_MANAGEMENT: PASS
CONFIG_SETUP_EDITOR: PASS
RAW_CONFIG_EDITOR: PASS
CONFIG_VALIDATION_PREFLIGHT: PASS
LIST_CRUD_AND_SOURCES: PASS
STRATEGY_REGISTRY: PASS
UPSTREAM_STRATEGY_CORPUS: PASS
CANDIDATE_LIBRARY: PASS
ZAPRET_IMPORT_PARITY: PASS
CLIENTHELLO_GENERATE_VALIDATE: PASS
CLIENTHELLO_CAPTURE_IPV4: PASS
CLIENTHELLO_CAPTURE_IPV6: PASS
BLOB_MANAGEMENT: PASS
TARGET_DIAGNOSTICS: PASS
TCP16_REAL_PROBE: PASS
TCP16_NETWORK_SNI_MEMORY: PASS
OBSERVED_TARGETS: PASS
DEVICE_TO_DIAGNOSE_HANDOFF: PASS
SANDBOX_TCP: PASS
SANDBOX_QUIC_UDP: PASS
SCORE_ENGINE: PASS
TARGET_MEMORY: PASS
INSIGHTS: PASS
HISTORICAL_RECOMMENDATIONS: PASS
RECOMMENDATION_AWARE_PLANNING: PASS
LIVE_WINNER_VERIFICATION: PASS
GENERIC_CANDIDATE_PREVIEW: PASS
GENERIC_CANDIDATE_SAFE_APPLY: PASS
ROLLBACK: PASS
JOBS_SCHEDULER: PASS
RUN_HISTORY: PASS
BACKUPS_RESTORE: PASS
GEO_LIST_IMPORT: PASS
WEB_NO_DEAD_ACTIONS: PASS
RESPONSIVE_UI: PASS
PRODUCTION_SAFETY_INVARIANTS: PASS
FINAL_HARDWARE_ACCEPTANCE: PASS

NFQWS_WEB_COMPLETE: PASS
```

At that point routine SSH/manual file editing is no longer part of the supported NFQWS user workflow.
