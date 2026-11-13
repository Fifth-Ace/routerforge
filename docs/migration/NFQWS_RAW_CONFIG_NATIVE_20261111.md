# RouterForge ALL11 — NFQWS raw `nfqws2.conf` Svelte ownership

**Base:** `Fifth-Ace/routerforge` / `dev` / `d069c1f09f95392368ab0e128857bc5a67e96c75`.
**Scope:** isolated source checkpoint, no Git writes, CI dispatch, publishing or router actions.

## Real production changes

- `NfqwsRawConfig.svelte`: native Svelte 5 sole UI/state owner of the **active** `nfqws2.conf` text editor, dirty state, gutter, syntax highlight, diff, confirmation, validation status, Lua docs and selected `--lua-desync` payload fix.
- `NfqwsRawConfigModel.js`: pure diff/stats/status policy, strict path and local SHA freshness checks. HTTP only on explicit button action; no fetch on import or mount, cancellation on unmount, single in-flight operation.
- `NFQWSManagerApp.svelte`: mounts new editor in the previous `view-config` panel; authoritative backend status enters via one callback. `NfqwsRuntime.js` is no longer allowed to read `$('config')`, write raw config HTML, or attach raw editor click handlers. It still owns shared Setup/Smart Apply commands. Smart Apply now projects the active-config state through the Svelte callback rather than writing the textarea.
- Preserved endpoints: GET `/status`, POST `/config/validate`, GET `/config/lua-catalog`, POST `/config` with existing `confirm:'NFQWS_CONFIG'` and server-side backup, atomic write, reload/rollback. No new server endpoint or mutation on mount.
- Server API lacks guaranteed atomic `If-Match` for active config. A `GET /status` and exact `config_sha256/content/path` check **immediately before** POST reduces stale writes but does **not** eliminate a server race; RT/LITE/NFQWS G6 acceptance remains open.

## Evidence and blocking gates

- Local dynamic Node model tests + previous NFQWS/DNS Lite tests: 68 + 22 = **90/90 PASS** (at this snapshot). `node --check` for modified JS PASS.
- Remote branch SHA checked read-only, unchanged. Baseline file changes limited to previous allowlist; 3 new frontend source/test files added.
- Pinned Vite production build of **this new slice**: **PENDING**, because Linux npm cache lacks `zimmerframe@1.1.5`; Windows `RUN_NATIVE_OWNER_PS51.ps1` performs real pinned npm ci and Svelte 5 build.
- Browser mount/update/unmount, active-config end-to-end Save/Validate/Lua, Linux Go/race/vet, real IPK/upgrade: **PENDING**.

## Not transferred / MUST NOT claim MODULE DONE

- NFQWS list/hostlist/ipset editor with inspect/analysis/rename/domain-check flows.
- NFQWS Strategy Library, registry/import, DPI/Sandbox/Selector and other remaining active imperative pages.
- DNS Lite active command bridge, behavioral/IPK acceptance and ALL11 G0–G8.

**Next development slice:** native NFQWS list/hostlist/ipset UI+commands, then remove active imperative owner, perform consolidated browser and backend gates. No automatic commit/push/CI/publish/router.
