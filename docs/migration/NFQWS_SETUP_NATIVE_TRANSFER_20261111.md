# NFQWS Setup Svelte owner transfer — 11.11.2026

BASE SHA: `d069c1f09f95392368ab0e128857bc5a67e96c75`, dev only.

## Production changes
- `NFQWSManagerApp.svelte`: one Svelte authority for all 12 editable setup fields, MODE_LIST/AUTO/ALL and policy choices, custom block text/labels/order, inventory refs. Bound inputs and derived UI replace imperative DOM form assignments.
- `NfqwsArgumentField.svelte`: native Svelte syntax-highlighted textareas for BASE_ARGS, TCP, QUIC, UDP and IPSET; read-only resource hints validated before navigation.
- `NfqwsCustomStrategies.svelte`: add/edit/clone/reorder/delete custom strategies, safe label metadata, same existing NFQWS_ARGS_CUSTOM serialization (including `--new`), no HTML string composition for cards.
- `NfqwsFlowModes.svelte`: domain and Keenetic policy choices are reactive; raw mode definitions are never changed, a blank policy is not considered safe to apply.
- `NfqwsRuntime.js`: no longer reads/mutates migrated setup inputs and custom cards via document; explicitly reads Svelte snapshots at draft/validation boundary. Existing server CAS/save/rollback and confirmation unchanged.

## Safety
- No router writes on mount or on field typing, only explicit config diff/apply flow.
- Flow hydration preserves original booleans and metadata until user changes them; no implicit forced normalization.
- Invalid mode names and resource names fail closed. UI disabled while service operation is active.
- No backend, package, ndnproxy, firewall or router changes.

## Verification
- Node dynamic tests: 71/71 PASS; both DNS Lite source gates PASS.
- Current Vite production and real browser: PENDING until Windows runner. Linux npm cache cannot supply pinned dependencies, no false PASS.
- N03–N09 MODULE DONE: PENDING — raw config, list file, strategy library and other legacy editors remain.
