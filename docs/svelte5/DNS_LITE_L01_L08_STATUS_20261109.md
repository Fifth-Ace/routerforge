# RouterForge ALL11 / DNS Lite L01–L08 — progress checkpoint

Base: 7bc931b30bd67e6d58013d1e12f24b5d605c4078 (dev), merged R15+Runtime lifecycle source, 923 files before this checkpoint. No Git operations or publishing.

## What changed

- L01: single `mount(DnsLiteController)` entrypoint; portal-owned children remain in the original layout. The seven independent Svelte root mounts were eliminated.
- L02: DNS cards receive reactive view props and guarded direct controller commands (edit, change, manual health check, fallback, delete) without permanent `window.__rfDnsLite*` bridges.
- L03: native domain-list editor receives a snapshot and direct actions. Rules stay draft-only until explicit save/apply. Legacy editor DOM is hidden but still supplies underlying form inputs; this is **not** fully native ownership yet.
- L04: mode and status read from the owner. No second component-owned service-status poll or mode listener; refresh explicitly GET-only. Owner handles periodic read-only status polling.
- L05: one trace polling path through the owner, with bounded log view. The separate Svelte log fetch/pause capture handlers were removed.
- L06: Settings uses a scoped Svelte context instead of global document mutation events. P21/P22 and P25 layout code still contain imperative DOM handlers and need migration.
- L07: `DnsLiteApi.js` centralizes JSON/same-origin policy, explicit action headers, mandatory CAS revision for PUT config, timeout, and in-flight cancellation after unmount. Guarded controller save does not clear dirty if draft changes during PUT.
- L08: 22 historic DNS UI tests and 14 P22 tests were preserved byte-for-byte under frontend-tests/historical. The 22 UI tests now run against the actual production Svelte IIFE rather than source-extracted controller text. Five new P22 browser behavior tests plus the native bundle test were added. A new Node API/security test suite covers 11 actual client behaviors.

## Evidence / not evidence

- 58/58 Node tests PASS in the creation environment (47 earlier DNS Runtime tests + 11 DNS Lite API tests). `bash -n` for the packaging script PASS.
- Source-derived `window.__rfDnsLite*` global bridges are absent in the edited Lite frontend.
- **NOT RUN HERE:** Svelte compiler, DNS Lite Vite build, 28 jsdom browser tests, Chrome parity, Lite Go tests, Linux race, canonical IPK, upgrade/rollback, exact-SHA CI.
- Do not infer functionality from source markers. The first Windows gate must compile and run the 28 actual bundle browser tests, then stop at first genuine FAIL.

## Known blockers for MODULE DONE

1. The 700+ line imperative portions of `DnsLiteController.svelte`, plus P21/P22 and P25 layout mutation paths, must be replaced with reactive owned state; one mount alone does **not** constitute 9/11.
2. The initial Lite behavior suite has not been run against this new bundle; regressions may appear and must be fixed in source, not bypassed.
3. Error/timeout, CAS race, Gate preflight/enable/disable, service ON/OFF, DNS/TLS and all nine IP Intelligence strategies must pass actual browser parity.
4. Linux Go/vet/race, real IPK, upgrade/rollback and later separate authorized exact-SHA nonpublishing CI are pending. No router modification without separate approval.

## Next safe gate

Use `RUN_DNS_LITE_L01_L08_PS51.ps1` from the containing archive. It validates the full 923-file previous manifest and 929-file target manifest, builds/tests the archive snapshot **before** applying the exact delta, backs up every overwritten file, refuses unrelated changes, and never runs Git/publish/router commands. No hidden global tool installation.
