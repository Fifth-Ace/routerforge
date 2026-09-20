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