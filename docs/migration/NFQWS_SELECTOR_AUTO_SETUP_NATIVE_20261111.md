# RouterForge ALL11 / Selector V2 + Auto Setup — native Svelte transfer, 2026-11-11

**Exact base:** `Fifth-Ace/routerforge` `dev` at `d069c1f09f95392368ab0e128857bc5a67e96c75`. Consolidated, isolated source; no git writes, CI dispatch, release, router, DNS, Keenetic mutations.

## Actual production ownership transfer

- `NfqwsSelector.svelte` + `NfqwsSelectorModel.js`: new owner of Selector V2 form, source selection (production, built-in pool, library, optional community ASN, manual argv), live `/v2/selector` POST, abort, bounded session polling, recommendation and candidate metrics, target profile binding via `/v2/selector-bind`, explicit exact Preview and manually confirmed `/bench-autotune/apply`.
- `NfqwsAutoSetup.svelte` + `NfqwsAutoSetupModel.js`: new owner of manual/observed/list target selection, DNS/TCP/TLS/HTTP diagnosis, optional TCP16 probe branch, stage timeline, Selector handoff, deterministic preview, safe apply verification and cancellation.
- `NFQWSManagerApp.svelte`: both screens are mounted Svelte components. The Selector registers an explicit controller only to support Auto Setup (not DOM writes). Domain/Observed Targets seed targets via Svelte props.
- `NfqwsRuntime.js`: previous Selector/Auto Setup `innerHTML`, selector polling, UI mutation and `.onclick` handlers eliminated. Retains shared same-origin API transport, history, status and other still-legacy workspaces.

## Safety invariants retained

- No mutation on mount/selection. Selector requires exact active configuration SHA and explicit `ROUTERFORGE_V2_SELECTOR`.
- Selector's live result must pass the server apply gate, or bind a verified WORKING candidate using `ROUTERFORGE_V2_SELECTOR_BIND`.
- Preview sends `ROUTERFORGE_AUTOTUNE_PREVIEW` with active SHA. Apply requires human confirmation, re-fetches status, compares SHA, sends exact receipt token, active/candidate SHA and `ROUTERFORGE_AUTOTUNE_APPLY`. Server remains authority for backup, rollback, post-apply proof.
- Selector community ASN requires opt-in with auto pool. Auto Setup does not silently opt in to community lookup.
- Abort on Stop/reset/unmount, poll interval cleanup, stale async generation fences. Read-only error/metrics rendered as escaped Svelte text.
- Auto Setup short circuits on healthy baseline, DNS or routing failures, and refuses applying when cleanup or live candidate proof is missing.

## Local executed evidence

- NFQWS **135/135** Node tests PASS, including 24 new Selector/Auto Setup tests.
- DNS Lite **22/22** Node tests PASS, unchanged.
- All changed `.js` and Svelte script syntax PASS.
- `scripts/verify-dns-lite-svelte-controller.py` PASS and `scripts/verify-p195-dns-ui.py` PASS.
- Pinned npm/Vite on this Linux environment BLOCKED by uncached `zimmerframe@1.1.5`, not a product source compile error.
- Windows runner includes real `npm ci` and production `vite build` for NFQWS and DNS Lite and must report actual result; real Chromium/API/IPK/upgrade still PENDING.

## Not yet finished

- NFQWS: Domain / Production Test Lab, DPI Detector / ClientHello, strategy registry/import, observed targets / target memory, history, blobs/catalog and other remaining legacy DOM. N03–N09 MODULE DONE PENDING.
- DNS Lite: active API/command bridge and browser/upgrade acceptance PENDING.
- Runtime RT-C4 production QUIC/IPK/upgrade and other 8 module behavioral acceptance PENDING.
- Server config-library CAS is not added; never call read-before-write an atomic server-side CAS.

**All11 MODULE DONE: PENDING.** No commit, push, release, CI or router action.
