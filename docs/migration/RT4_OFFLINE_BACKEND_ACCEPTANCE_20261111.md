# RouterForge / RT-C4 exact-source offline backend acceptance (2026-11-11)

**BASE**: Fifth-Ace/routerforge / dev / `d069c1f09f95392368ab0e128857bc5a67e96c75`.
**Mode**: isolated local copy. No remote branch mutations, publishing, CI dispatch, router, ndnproxy/stubby, Entware, firewall or DNS changes.

## Executed directly on Linux (Go 1.23.2, linux/amd64)

1. Real `go test -tags dns_router_lite -count=1 -json` on `./modules/dns-router-runtime/...` and `./components/core`: **PASS, 630 test/subtest pass events, 0 test failures**. This includes 3 additional go tests in `rt4_upgrade_recovery_test.go`:
   - `TestRTC4OfflineRestartStaysOfflineAndPreservesRevision`: OFF→OFF after backend reconstruction, read-only API, no ownership ledger, unknown ownership not disabled;
   - `TestRTC4ActiveRestartPreservesOwnedLedgerAndCanRestoreBaseline`: active ledger and revision persisted across reconstruction, stale ledger CAS refused, explicit disable restores exact NDMC mock baseline;
   - `TestRTC4RestartForeignDriftRequiresManualReview`: a foreign zone blocks disable, owned ledger is retained, foreign DNS remains untouched.
2. Real `go test -race -tags dns_router_lite -count=1` for the same packages: **PASS**; `go vet -tags dns_router_lite`: **PASS**.
3. Actual Linux backend process spawned with an isolated test-only `dns_router_lite` binary, an ephemeral Unix socket and temporary config. `qa-runtime/rt4_offline_process_e2e.py` executes four groups: **4/4 PASS** — clean startup GET has no active listener nor persistent config; unauthorized/missing-CAS writes 403/428 cause no mutation; CAS PUT 200, stale 409; SIGTERM and new backend process retain saved config while remaining OFF. No real ndmc invoked.
4. Compiled test-only Go binaries for **aarch64, MIPS BE, MIPS LE**, validated ELF machine and endianness. All three built without QUIC via the project-defined `dns_router_lite` tag; this is **not** proof of the production binaries or experimental IPK.

Logs and checksums: `evidence/RT4_EXECUTED_20261111/`. Test-only ELF binaries are intentionally excluded from the deliverable archive; their checksums are recorded in `crossbuild-sha256.txt`.

## PENDING / BLOCKED

- Full production (untagged) `go test`, `-race`, `vet`: **BLOCKED**; pinned `github.com/quic-go/quic-go v0.46.0` missing offline and `proxy.golang.org` DNS cannot resolve.
- Real experimental aarch64/mips/mipsel IPK packaging: **NOT RUN**. The isolated tag-only cross-compiles do not count.
- Real `opkg` upgrade OFF→OFF and ON→ON, rollback and restart integration: **NOT RUN**. Backend-process restart and simulated NDMC owner ledgers are meaningful but **not an IPK upgrade**.
- Full all-11 module behavioral audit and `MODULE DONE`: **PENDING**. No acceptance granted by Node or Go test counts alone.

## Next nonpublishing execution

`RUN_RT_C4_LINUX_NONPUBLISHING.sh` is a self-contained Linux test runner. It first checks package hashes and current exact remote SHA, then executes independently available lite-tag and real Unix-process evidence, follows with **untagged** production Go/race/vet and 3 experimental IPKs. It fails closed and retains logs at first real FAIL or missing dependency. It does not install packages or mutate the router. Production pass and IPK pass are required before RT-C4 can advance.
