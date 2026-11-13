# RouterForge ALL11 — NFQWS DPI Web/NDJSON Svelte ownership (2026-11-12)

BASE: `Fifth-Ace/routerforge/dev` / `d069c1f09f95392368ab0e128857bc5a67e96c75`. This is a local, consolidated, nonpublishing development checkpoint. It follows the Registry+DPI Status checkpoint; do not overlay old archives.

## Functional source changes
- `NfqwsDPIWorkspace.svelte` owns DPI Detector console iframe, overlay/restart, seven Web test choices, all former run form fields, progress, explicit Start/Stop, upstream legend, Native Live NDJSON presentation and copy of the original report as escaped text. It performs a *single* explicit stream POST only after a GET `/status` yields a valid active SHA.
- `NfqwsDPIWorkspaceModel.js` owns exact Go request contract with confirm `ROUTERFORGE_DPI_DETECTOR_V5_RUN`, bounds for tests/targets/options, SHA verification and text-only report section navigation. No code in the model mutates network/DOM.
- Success requires a `done` event with `production_config_unchanged===true` AND an exact matching `production_config_sha256`. `chunk` or a truncated stream alone cannot be counted as success. Abort/mount and tab-change lifecycles prevent UI state updates after cancellation. Backend already owns active SHA, command context, streaming timeout and compatibility gates.
- Legacy `NfqwsRuntime.js` no longer reads or writes `dpiWeb*`, console and Native Live DOM controls and no longer renders the report with `innerHTML`. It retains only read-only status GET `/dpi-detector` for the separate Svelte status panel/Keenetic view. No backend/route/active-config changes. `dpiRefresh()` is a read-only bridge.
- The native Web result intentionally preserves the *full upstream console report* and read-only section navigation, rather than claiming that the historical rich visual result cards have parity. Visual result-card parity is an explicit remaining browser acceptance requirement.

## Verification and limits
- Linux Node 22 regressions: NFQWS **199/199 PASS**, DNS Lite **22/22 PASS**. Six new DPI model/owner tests PASS; exact existing NDJSON chunk/403/cancellation tests remain PASS.
- Linux pinned Vite **BLOCKED**: npm cache lacks `zimmerframe@1.1.5`; no network/install/repository mutation performed. **Windows pinned Vite and zero-warning Svelte gate PENDING**, and no browser equivalence for the newly moved workspace has yet been confirmed.
- Processing Map (own isolated fast-test NFQUEUE workflow), other NFQWS legacy screens, DNS Lite acceptance, production Go/IPK/upgrade and server CAS are **PENDING**. NO `MODULE DONE` or 11/11 signoff.
- Commit/push/CI dispatch/main/tags/release/Router/Keenetic: UNTOUCHED.
