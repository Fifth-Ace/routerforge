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
