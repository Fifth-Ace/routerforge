# RouterForge ALL11 / Svelte 5 — текущий рабочий срез, 11.11.2026

BASE: `Fifth-Ace/routerforge`, `dev`, exact `d069c1f09f95392368ab0e128857bc5a67e96c75`. Remote SHA проверен read-only, no drift. Рабочее дерево — автономная копия. Commit/push/CI/release/router запрещены.

## Результат миграции
- Svelte 5 source/entry существуют во всех 11 модулях. Это **не MODULE DONE**.
- DNS Runtime: RT-C3 на Windows: 34/34 реальных Chromium browser cases PASS; RT-C4 полное Go QUIC/IPK/upgrade PENDING/BLOCKED.
- DNS Lite: перенесены редактор DNS, 38+ полей P25/P27–P31/P24x, 9 IP Intelligence modes, журнал и shell state в Svelte-owned. `DnsLiteControllerLogic.js` ещё активен как API/commands bridge и poll lifecycle; сохраняются статическая outer-shell и интеграция родителя. Behavioral/IPK upgrade gates PENDING.
- NFQWS: Svelte владеет навигацией, lifecycle, service state, logs, Lua Scripts, DPI stream, Overview, Backup Center, List Sources, Smart Apply и теперь всем setup-черновиком NFQWS: 12 form fields (WAN, TCP/QUIC/UDP/IPSET, ports, IPv6, policy/debug), MODE_LIST/AUTO/ALL, Keenetic policy, custom strategy block editor/labels/reorder/clone. `NfqwsRuntime.js` сохраняет CAS/commands и ещё владеет raw config/library/hostlist editors и другими legacy DOM screens. **MODULE DONE пока нет**.
- Остальные 8 модулей: Svelte source/entry, behavioral audit PENDING.

## Выполненная проверка этого среза
- 71/71 динамических Node тестов PASS для NFQWS+DNS Lite (включая field round-trip, modes, custom strategies, escaping, resources).
- `scripts/verify-dns-lite-svelte-controller.py`: PASS. `scripts/verify-p195-dns-ui.py`: PASS.
- JS syntax for all changed .js: PASS. Actual Svelte compiler/Vite in this Linux container BLOCKED: pinned npm packages are not present and registry.npmjs.org is unreachable.
- Предыдущий Windows RUN подтвердил pinned NFQWS+DNS Lite production Vite build на предыдущем source slice: PASS. **Этот новый срез требует Windows runner**, и не считается собранным до этого.
- NFQWS и DNS Lite `MODULE DONE`: **PENDING**. Commit/push/CI/router untouched.


## 11.11.2026 / NFQWS Config Library functional source transfer
- `NfqwsConfigLibrary.svelte` owns Config Library all controls/CRUD/diff, `NfqwsRuntime.js` cache-only for Strategy/Smart Apply. See `NFQWS_CONFIG_LIBRARY_NATIVE_20261111.md`.
- Local Node 58 NFQWS + 22 DNS Lite PASS, JS syntax PASS; new Svelte Vite/browser still PENDING, no module done.
- Backend config-library `If-Match` unsupported: client-side SHA preflight is NOT atomic CAS; required server signoff is BLOCKED until explicit backend authorization.

## 11.11.2026 / Native NFQWS raw config editor
- `NfqwsRawConfig.svelte` owns active nfqws2.conf input, diff, save-confirm, syntax validation, Lua docs and payload-fix UI. `NfqwsRuntime.js` no longer reads/writes `$('config')` or assigns its handlers. Smart Apply forwards refreshed status to Svelte.
- Dynamic NFQWS + DNS Lite Node models: 90/90 PASS. New slice pinned Vite and browser: PENDING (Linux npm cache missing zimmerframe). See `NFQWS_RAW_CONFIG_NATIVE_20261111.md`.
- Backend atomic CAS for `/config` not verified. Client SHA freshness check is NOT server CAS. Entire NFQWS `MODULE DONE`: PENDING.

## NFQWS native list editor transfer — 11.11.2026
- Hostlist/ipset List Editor core UI and create/save/delete/rename-preview/apply now owned by `NfqwsListEditor.svelte`; old imperative list CRUD/renderers removed. See `NFQWS_LIST_EDITOR_NATIVE_20261111.md`.
- Current local Node 98/98 PASS across NFQWS + DNS Lite. New pinned Windows Vite and browser acceptance PENDING. Auxiliary list diagnostics and Strategy Library still active legacy.

## NFQWS Strategy Library native Svelte transfer (11.11.2026)
- New `NfqwsStrategyLibrary.svelte` owns inventory/editor/new/save/delete/Sandbox handoff and all source values from Go backend; legacy Strategy Library DOM functions/handlers removed. Previous List Editor native transfer remains.
- Local post-transfer Node and syntax checks documented in package; pinned Vite/browser for current source **PENDING** until Windows runner. Atomic server CAS for strategy library not added. NFQWS N03–N09 still PENDING.

## 11.11.2026 / native NFQWS Selector V2 + Auto Setup
- `NfqwsSelector.svelte`, `NfqwsAutoSetup.svelte` own the complete UI, progress, target sources, live selector run, bind, preview and confirmed apply; legacy `#v2Selector*` and `#v2AutoSetup*` DOM owners removed. Shared API transport and other workspaces remain in runtime.
- 135/135 NFQWS and 22/22 DNS Lite Node local PASS; both Lite source gates PASS; changed JS/Svelte script syntax PASS. New pinned Windows Vite and browser/Go/IPK upgrade remain PENDING. See `NFQWS_SELECTOR_AUTO_SETUP_NATIVE_20261111.md`.
- No side effects on router or Git. MODULE DONE PENDING.
