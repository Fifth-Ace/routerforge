# ALL11 ownership follow-up / read-only source audit 2026-11-11

Source baseline: `Fifth-Ace/routerforge/dev@d069c1f09f95392368ab0e128857bc5a67e96c75` with the staged RT-C3/C4 test-only additions. This is an inventory, not completion signoff.

## DNS Lite L07/L08

- Entry `modules/dns-router-lite/frontend/main.js` mounts exactly one Svelte root (`DnsLiteController.svelte`) and unregisters on non-persisted `pagehide`; portals render Svelte views into the existing layout. This satisfies **source/entry**, not full behavior/state ownership.
- Active `DnsLiteControllerLogic.js` (846 lines) still performs DOM reads and writes: 177 `byId(` references, dynamic `replaceChildren`, manual `disabled`, form values, page visibility, feedback, settings mirroring, timers, resize and parent communication. It also drives REST/CAS and Gate. That is the **active imperative controller** preventing L07 MODULE DONE.
- Svelte owns cards, domain editor, logs, status, P21/P22/P25 components, but controller retains an imperative presentation and effects layer. Migrating just the entry count or renaming it would not meet L07/L08.
- Native pure JS logic tests `tests/dns_lite_*.mjs`: **40/40 PASS** in the current Linux environment. Historical DOM/UI suite requires `jsdom`, which is missing here; 10 test files failed to load due to environment (`MODULE_NOT_FOUND`), not confirmed product failures. No Svelte browser acceptance of DNS Lite was performed here.
- Essential behavioral acceptance after state-owner migration: independent lists per DNS, list selection preserving dirty draft, TLS certificate inspection, trace/log zero-before-open and pause, P21 devices, 9 IP-intelligence modes and health probes, RF Gate fail-closed, 403/409/If-Match/rollback/confirm, ON→ON/OFF→OFF isolated upgrade. All remain PENDING.

## NFQWS N03–N09

- `modules/nfqws-manager/frontend/NfqwsRuntime.js` is an active 3931-line legacy runtime. Source-level inventory contains 492 `function` declarations, 168 `innerHTML` uses, 21 `.addEventListener`, 22 `createElement` and mutable timers/network calls. `NFQWSManagerApp.svelte` provides entry/shell but cannot alone establish single native UI/state ownership.
- Required complete native pass: status/logs/modes; hostlist/ipset; strategies/editor/backups; DPI NDJSON stream and cancellation; Smart Apply/one-shot/rollback; browser/Go/IPK; fail-closed mount cleanup. PENDING.

## Safety conclusion

No Lite/NFQWS source edits performed during this RT-C4 continuation: the remaining work is a **whole-feature ownership migration**, not a cosmetic entry/source count correction. Do not mark ALL11 DONE.
