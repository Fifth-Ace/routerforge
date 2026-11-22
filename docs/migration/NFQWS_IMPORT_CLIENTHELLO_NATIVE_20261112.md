# RouterForge ALL11 / NFQWS Native Strategy Import + ClientHello / 2026-11-12

## BASE / TARGET
- Repo: `Fifth-Ace/routerforge`, branch `dev`, SHA `d069c1f09f95392368ab0e128857bc5a67e96c75`.
- This ZIP is a **complete consolidated repository/** tree; do not apply earlier ZIPs over it.
- Previous NFQWS native UI owner changes, DNS Lite and Runtime RT-C3/RT-C4 source remain intact.

## FUNCTIONAL TRANSFER (not just tests)
1. `NfqwsStrategyImport.svelte` + `NfqwsStrategyImportModel.js`: owns paste/file input, explicit `/v2/import/parse`, result warnings/transforms/dependencies, preview/diff, inactive `/configs/create`, explicit `/v2/strategies/import`, and exact-profile Sandbox handoff. Removed historical `renderStrategyImport`, `runStrategyImport`, file handlers, library-save DOM and legacy tokenizer/UI from `NfqwsRuntime.js`.
2. `NfqwsClientHello.svelte` + `NfqwsClientHelloModel.js`: owns capabilities, explicit 5-second capture, selection of valid ClientHello candidate, forge or synthetic generation, preview of blob parameters, **fresh inventory before install**, explicit replace/install confirmation and `/blobs/install`. Removed old ClientHello DOM handlers from runtime. Blob Manager read-only validate remains shared.
3. Svelte components own component lifecycle and AbortController cancellation on unmount / inactive screen. Values render as escaped text, not `innerHTML`.
4. Existing API and backend unchanged. Existing config `status` and SHA used in Svelte for a fail-closed, **read-before-write** inactive library save. Backend Config Library does **NOT** offer atomic CAS; this race remains an acceptance gap. Existing blob install endpoint has exact SHA and explicit install/replace token; it does not restart service.

## VALIDATION ACTUALLY RUN ON LINUX
- NFQWS Node 185/185 PASS; DNS Lite Node 22/22 PASS.
- `node --check` on all changed/new JS; parsed Svelte `<script>` portions using installed TS parser: 0 syntax diagnostics.
- Scope vs previous consolidated ZIP: exactly 2 changed, 6 added source files, no deleted files.
- No pinned Svelte/Vite build available in local offline Linux (npm cache missing dependency `zimmerframe`); **new build PENDING**, not PASS.
- `RUN_NATIVE_OWNER_PS51.ps1` performs exact source manifest + remote dev gates, Node tests, strict Svelte zero-warning pinned Vite builds of both NFQWS and DNS Lite, stopping on first real failure and producing diagnostics.

## NEVER AUTOMATED
- No git commit, push, CI dispatch, main/tags, release/publish, router activation, external nfqws restart or firmware changes.

## OPEN GATES
- NFQWS DPI Detector / Processing Map / remaining active legacy DOM owners.
- DNS Lite final command bridge migration + browser/API acceptance.
- RT-C4 production Go QUIC, real IPK and upgrade testing, general full browser parity, server Config Library CAS. **ALL11 MODULE DONE = PENDING.**
