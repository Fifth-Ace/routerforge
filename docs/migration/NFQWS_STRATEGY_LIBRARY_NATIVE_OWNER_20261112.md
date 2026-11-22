# NFQWS Strategy Library — Svelte-owned server inventory / 2026-11-12

## Symptom / baseline

`RUN_NATIVE_OWNER_PS51.ps1` stopped at `NATIVE_STRATEGY_OWNER_NOT_CONNECTED` after Node tests passed. The old Strategy Library route was removed from the controller but its `strategyLibraryRefresh()` method still called undefined `v2LoadLibrary(true)`; `onStrategyLibrary` was declared yet never invoked. This was a **functional connectivity defect**, not merely an obsolete test.

Exact remote `dev` reference: `d069c1f09f95392368ab0e128857bc5a67e96c75`. No changes to backend, main, tags, router or CI.

## Native ownership

- `NfqwsStrategyLibrary.svelte` performs GET `/v2/strategies` through the existing same-origin request bridge.
- The Svelte view owns the reactive inventory, first activation, manual refresh, Save/Delete rereads and error/empty states.
- `verifiedStrategyLibraryResponse` checks the actual Go HTTP shape: `ok`, `version=1`, `max=64`, exact count, valid IDs/sources/argv and SHA256 fingerprint syntax. Invalid or partial responses fail closed; no filtered fake success.
- `NFQWSManagerApp.svelte` increments `strategyLibraryRefreshSeq` on Strategy Import invalidation; the component refreshes on activation or increment, without a DOM command.
- Removed the obsolete `onStrategyLibrary` callback, `v2Library` cache and undefined `v2LoadLibrary` call from `NfqwsRuntime.js`.
- `RUN_NATIVE_OWNER_PS51.ps1` checks this new ownership path and rejects reintroduction of the dead loader. Source gates, pinned Vite and zero Svelte warnings remain mandatory.

## Executed here / pending

- Local NFQWS and DNS Lite Node tests: PASS (see evidence in archive).
- Full pinned Vite/Svelte 5 compiler on Windows: **PENDING** until executing the included PowerShell 5.1 runner. Do not label MODULE DONE from Node alone.
- Browser, Linux Go/IPK/upgrade, backend CAS of config library: remain separately pending.
