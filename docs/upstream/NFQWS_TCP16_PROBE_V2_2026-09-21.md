# RouterForge TCP16 Probe V2 — provenance and safety lock

Date: 2026-09-21  
RouterForge base SHA: `7b0018c57274d77d1138d93c2466009cc2ff6774`

## Upstream behavior reviewed

Repository: `necronicle/z2k`  
Pinned ref: `fca1ed5a452f2554b3dfa1ab18571cee7c505174`

Relevant upstream files:

- `z2k-detect/internal/tcp16/tcp16.go`
- `z2k-detect/cmd/z2k-detect/tcp16.go`
- `files/lists/tcp16_targets.txt`
- `files/lists/sni_wl_candidates.txt`

RouterForge adapts the controlled keep-alive volume probe concept:
ten HTTP HEAD requests on one connection, 4000-byte padding after the first request,
and classification only when a connection dies after the 12 KiB floor.

## RouterForge differences

RouterForge does not import z2k runtime scripts or modify NFQWS configuration.

- target set is a small pinned curated subset, bounded to six;
- default run uses four targets from different networks/providers;
- SNI scan is bounded to ten names and at most two detected networks;
- a working SNI is persisted only after two complete clear probe runs;
- production config SHA is checked before and after the active probe;
- if production SHA changes during the run, probe results are returned but memory is not persisted;
- the only write is RouterForge-owned TCP16 evidence memory under the existing target-memory root;
- no service restart/reload, NFQUEUE rewrite, config edit, route edit or firewall edit occurs.

This stage produces evidence only. It does not automatically apply an SNI bypass.
