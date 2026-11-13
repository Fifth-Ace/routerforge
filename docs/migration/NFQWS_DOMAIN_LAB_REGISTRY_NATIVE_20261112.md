# NFQWS Svelte 5 — Domain Test / Production Test Lab / TCP16 Memory / Strategy Registry

Base: Fifth-Ace/routerforge `dev` `d069c1f09f95392368ab0e128857bc5a67e96c75`.
Mode: isolated source checkpoint, no Git writes, no publishing, no router changes.

## Functional ownership transfer

- `NfqwsDomainTest.svelte`: explicit `/v2/site-diagnose` POST on user action; stages DNS/TCP/TLS/HTTP, current list/profile binding, diagnostic context and a manual Selector handoff. Status/draft/error and abort-on-unmount owned by Svelte. `onDomainSeed` preserves the Observed Targets → Diagnose transition.
- `NfqwsProductionLab.svelte`: target source inventory GET when active; explicit `resolve` and `run`; `/v2/targets/resolve`, `/v2/inspect-target`, `/v2/detect` with bounded rounds/workers and abort-on-Stop/unmount; progress/history, read-only results. Guided Troubleshooter passes target through `onLabSeed`, no DOM field writes.
- `NfqwsTCP16Memory.svelte`: read-only `/v2/tcp16-memory`, stateful refresh, suspected/clear/Working SNI counters and history.
- `NfqwsStrategyRegistry.svelte`: five exact GETs, fingerprint-joined scores/insights/automation/recommendations, filters, evidence and historical ranking. Strategy Import invalidates through `onRegistryInvalidate`; no legacy `innerHTML` owner.

Existing backend endpoints are unchanged. The first two diagnostics initiate read-only network probes *only after explicit user Run*; neither changes the active strategy or operating system. Strategy Registry and TCP16 Memory perform GET only.

## Validation

- Node behavior: 159 NFQWS + 22 DNS Lite = 181 PASS, including 13 newly authored tests for these screens.
- `node --check` on changed JS and all existing frontend source tests PASS.
- Existing current Windows Vite evidence **before this patch**: 2026-11-12 00:48, both frontends PASS with zero Svelte diagnostics. This is not a build of this patch.
- This patched production Vite: **PENDING** on Windows; Linux offline npm fails `ENOTCACHED: zimmerframe@1.1.5`.
- Browser/API integration for the four screens: PENDING. No MODULE DONE signoff.

## Open migration / acceptance

NFQWS still has active legacy DOM paths, especially DPI Detector, ClientHello, strategy import, processing map and adjacent workspaces. DNS Lite requires final controller-removal/browser acceptance. Runtime RT-C4 production QUIC/IPK/upgrade remains pending. Backend library CAS race remains open. Do not publish or install this checkpoint.
