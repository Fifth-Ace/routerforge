# RouterForge ALL11 — actual Svelte functional migration slice (2026-11-11)

BASE: Fifth-Ace/routerforge dev `d069c1f09f95392368ab0e128857bc5a67e96c75`. Not committed/pushed.

## Implemented — production source changes

### NFQWS Manager
- Svelte now owns selected view for **30 screens**, 27 subnavigation and 6 primary group buttons, group strip visibility, and last-used group tab selection.
- Navigation invoked by existing business workflows calls a narrow `onNavigate` callback into the Svelte state rather than dispatching a document event and toggling DOM classes.
- Svelte owns top-level service status/target/subtitle, notice text/tone, busy/disabled conditions, and Start/Stop/Reload/Restart/Refresh button event handlers. Existing guarded backend action/confirmation code remains unchanged.
- Lua Scripts read-only inventory, viewer, search, copy and fullscreen are moved into `NfqwsScripts.svelte` and a DOM-free read-only API boundary; existing setup deep-link routes into the Svelte viewer.
- Logs screen is now `NfqwsLogs.svelte`; filtering, severity, statistics, raw copying and escaping are Svelte-owned, with a pure `NfqwsLogModel.js` and programmatic guided-search handoff.
- DPI Detector v5 NDJSON transport is now a standalone DOM-free command with explicit POST, timeout/cancel-aware stream parsing, bounded line size and guarded URL.
- Mount returns a `dispose` function, invoked by Svelte on unmount. It aborts API calls and the DPI NDJSON stream, V2 Lab/Sandbox/Selector, clears timer polling, disconnects ResizeObserver, and unregisters document/window listeners.

### DNS Lite
- The live feedback is now Svelte-rendered through a single UI portal, not imperative `textContent` writes.
- Service/Gate/connection control eligibility and labels are reactive Svelte props (`ui.controlView`) instead of controller DOM `disabled`/`textContent` writes for those buttons.
- Existing Svelte navigation buttons own selected/ARIA state; the controller no longer toggles their DOM classes.

## Explicitly NOT done
- **NFQWS:** `NfqwsRuntime.js` still owns the imperative editor/render, strategies, hostlists/ipset, Smart Apply and recovery, DPI presentation; it must be replaced by smaller state/command modules + Svelte components, not renamed or deleted blindly.
- **DNS Lite:** `DnsLiteControllerLogic.js` still owns imperative legacy editor, gate, logs, TLS and settings DOM; L07/L08 not signed off.
- **Runtime:** RT-C3 34/34 browser PASS in prior exact Windows checkpoint; RT-C4 production QUIC/IPK/upgrade pending.
- **Full Svelte build/browser tests for THIS new slice:** PENDING Windows gate; Svelte compiler packages unavailable in current offline environment. No claim of MODULE DONE.

## Safety
No backend modifications, no new permissions, no changes to Keenetic/Entware/ndnproxy/stubby/Gate/firewall, no commit/push/CI dispatch, no release or install. The new UI invokes the existing protected NFQWS `action()` path for mutations.

## Next
Perform Windows PS5.1 native builds of both frontends. Then continue substantive DNS Lite UI/state migration and NFQWS editors+streaming/Smart Apply; run real browser and API integration against isolated mocks, then Linux production Go/IPK/upgrade prior to MODULE DONE.
