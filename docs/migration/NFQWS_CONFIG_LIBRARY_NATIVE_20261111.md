# NFQWS Config Library — native Svelte 5 transfer (11.11.2026)

BASE: `Fifth-Ace/routerforge` dev `d069c1f09f95392368ab0e128857bc5a67e96c75`. Isolated local source; no Git writes, CI dispatch, release, router or backend change.

## Implemented, actual source ownership

- `NfqwsConfigLibrary.svelte` owns full Config Library panel, file list/search, neighbor files, editable raw config, syntactic preview, line numbers, diff confirmation, name modal and fullscreen. No `innerHTML`/DOM delegated events from `NfqwsRuntime.js` for this screen.
- Explicit user commands: create empty/from current, read, backup-save, rename, duplicate, import neighbor `.conf/.cfg`, backup-delete, activate. Exact existing `NFQWS_CONFIG_LIBRARY_*` confirmation strings preserved.
- `NfqwsConfigLibraryModel.js` is pure argument/endpoint normalization, safe diff, format and preflight SHA validation. `NfqwsConfigSyntax.js` escapes arbitrary content before highlighting in Svelte.
- On unmount, AbortController cancels outstanding HTTP, sequence guards prevent stale display, action busy-lock prevents double-click. No POST on import or mount.
- Existing Strategy Catalog + imported-strategy navigation uses `onConfigOpen` -> Svelte selection; old cache (`loadConfigLibrary`) is read-only and no longer modifies Config Library UI.
- `NfqwsRuntime.js` no longer binds `storedConfig*` event handlers, modal events or direct editor mutations; its other panels and business commands remain until next functional slices.

## Security/behavioral boundaries

- The existing Go endpoints `/v1/configs/*` do **not** support atomic server-side `If-Match` for stored configs. Before modifying an existing file, the new UI performs a `GET /configs/read` and exact `sha256` check, then makes a **single** explicit POST. This is a best-effort stale-read defense, NOT a server-side CAS guarantee: external concurrent writers can race in the gap. Endpoint CAS/If-Match remains BLOCKED until backend mutation is separately authorized.
- Config activate still uses the existing Go backend's backup, validation, atomic switch and restart/rollback contract. No changes to these server handlers or scripts are included.
- Dirty library editor is not discarded by silent inventory refresh or opening creation dialog. File removal detected after refresh retains unsaved draft and raises explicit warning.
- Reactively active page fetches GET inventory; no automatic mutations and no router side effects from mount.

## Evidence and remaining gates

- Linux local `node --test`: NFQWS 58/58 PASS, DNS Lite 22/22 PASS, including 9 new Config Library unit tests (real request/403/409/428/Abort/diff/SHA/name cases).
- JS syntax `node --check`: PASS for current NFQWS .js files.
- Full Windows pinned Svelte/Vite compiler for **this new component**: PENDING. Linux `npm ci --offline` BLOCKED (`zimmerframe@1.1.5` not cached). Do not claim build PASS until runner completes.
- Real browser/API integration including file save/activate rollback, UI visibility, mobile and unmount: PENDING.
- NFQWS remaining raw active `nfqws2.conf` editor, hostlist/ipset file editor, remaining strategy/diagnostic panels and legacy controller: PENDING. No NFQWS MODULE DONE/ALL11 DONE.
