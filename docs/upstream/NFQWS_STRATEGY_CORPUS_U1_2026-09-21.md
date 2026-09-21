# RouterForge NFQWS Strategy Corpus — U1 provenance lock

Date: 2026-09-21  
RouterForge base SHA: `a77cdb56d8527cbde4e277c5574cfdd21243d1ce`

This file records the immutable upstream basis for the static RouterForge NFQWS candidate corpus introduced by U1.

## Omn1z source

- Repository: `Omn1z/nfqws2-keenetic-strategy-selector`
- Ref: `bf4e810ef22ffb6671e97dc411234ba9430909c9`
- Source file: `internal/services/strategy/core/catalog/strategy.go`
- RouterForge adaptation: resource-free TCP/TLS and HTTP candidates from `AutoCandidates()`.
- Excluded in U1: candidates that depend on named blobs/pattern resources such as `tls_clienthello`, because U1 must not silently install or assume optional resources.
- RouterForge destination: `modules/nfqws-manager-runtime/strategy_corpus_v1.go`.

## z2k source

- Repository: `necronicle/z2k`
- Ref: `fca1ed5a452f2554b3dfa1ab18571cee7c505174`
- Source files: `quic_strats.ini`, `lib/strategies.sh`.
- RouterForge adaptation: resource-free QUIC techniques split out of the `quic_autocircular` pool as individually testable candidates.
- Excluded in U1: z2k-specific Lua morph helpers and named blob candidates that require resources not guaranteed by the installed production nfqws2 base args.
- RouterForge destination: `modules/nfqws-manager-runtime/strategy_corpus_v1.go`.

## Safety differences

RouterForge does not import an upstream strategy into production configuration. The static corpus is read-only candidate data. Every candidate:

1. is normalized through RouterForge's portable candidate path;
2. must compile through `v2CustomProfileForTransport`;
3. is deduplicated by deterministic technique fingerprint;
4. enters only the bounded candidate pool;
5. must pass fresh isolated NFQUEUE live verification before it can become a recommendation;
6. can reach production only through the existing exact-SHA Preview -> explicit Smart Apply transaction.

No U1 code edits production NFQWS configuration, restarts production, or downloads upstream resources at runtime.
