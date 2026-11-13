# UI and state owner map

## DNS Lite
- `DnsLiteController.svelte`: mounted Svelte root and action wiring.
- `DnsLiteControls.svelte`: server selector/new-DNS/TLS probe UI, manual client MAC + persistence choices, visibility/state controls.
- `DnsP25Settings.svelte` + `DnsLiteSettingsModel.js`: reactive 38+ settings fields and deterministic config projection, CAS server API remains in command logic.
- `DnsLiteLog.svelte` + `DnsLiteLogModel.js`: log filtering/visibility; no imperative filter ownership.
- `DnsLiteShellState.svelte` + `DnsLiteShellBridge.js`: parent shell projection, theme, resize, observers and cleanup at unmount.
- `DnsLiteControllerLogic.js`: **active commands**, network, CAS/rollback. DOM form access removed but controller still present; MODULE DONE not accepted.

## NFQWS Manager
- `NFQWSManagerApp.svelte`: navigation for 30 pages, service state/action control, Svelte-owned selected UI data.
- `NfqwsLogs.svelte`, `NfqwsScripts.svelte`: logs, Lua read-only inventory/viewer.
- `NfqwsDpiStream.js`: detached streaming API, cancellation on unmount.
- `NfqwsBackupCenter.svelte`, `NfqwsBackupModel.js`: backup list/search/diff/restore UI; backend mutation only via explicit safe commands.
- `NfqwsListSources.svelte`, `NfqwsListSourceModel.js`: local/upstream list catalog, SHA status and explicit install command.
- `NfqwsSmartApply.svelte`, `NfqwsSmartApplyModel.js`: exact-SHA plan, UI, visual state; server transaction/rollback remains unchanged.
- `NfqwsOverviewModel.js`: overview state computations.
- `NfqwsCustomStrategies.svelte` / `NfqwsCustomStrategiesModel.js` / `NfqwsCustomSyntax.js`: own custom strategy CRUD, ordering, labels and safe syntax highlight. `NfqwsRuntime.js` reads immutable snapshots only to build existing config/CAS diff. No imperative custom card renderer/click listeners.
- `NfqwsFlowModes.svelte` / `NfqwsFlowModeModel.js`: own MODE_LIST/AUTO/ALL and policy radio controls. Upstream MODE_* definitions never rewritten by UI choice.
- `NfqwsArgumentField.svelte` / `NfqwsFlowFieldsModel.js`: Svelte owns all 12 setup fields and read-only resource hints. No legacy DOM reads/writes for setup inputs; full configuration remains staged until explicit save.
- `NfqwsConfigLibrary.svelte` / `NfqwsConfigLibraryModel.js` / `NfqwsConfigSyntax.js`: own Config Library inventory/search, stored editor/diff, create/import/rename/duplicate/save/delete/activate; only cache-only reads remain in Runtime for Smart Apply/Strategy. Existing backend lacks atomic config-library CAS (PENDING).
- `NfqwsRuntime.js`: **remaining active business commands AND raw active config, hostlist/ipset editors, strategy library and other legacy DOM screens**. Must be decomposed/replaced before FULL N03–N09.

### NFQWS raw active config (11.11.2026)
- `NfqwsRawConfig.svelte`: single DOM/UI owner for `view-config`, dirty draft, diff, syntax-check, Lua catalog and confirmed Save.
- `NfqwsRawConfigModel.js`: pure immutable model, four allowlisted API routes with explicit invocation and abort.
- `NfqwsRuntime.js`: active backend status and shared Setup/Smart Apply operations; sends status via `onRawConfig`, not raw DOM writes. **Still legacy owner for List Editor and numerous other panels.**

## NFQWS List Editor (new owner)
- `NfqwsListEditor.svelte` + `NfqwsListEditorModel.js`: inventory, search, list editor, stats/syntax, normalization, dirty state, CRUD and transactional Rename Preview/Apply. No legacy DOM mutation for these controls.
- `NfqwsRuntime.js`: read-only list snapshot bridge for record inspector, static path and cross-list diagnostics. These auxiliary DOM views still need Svelte migration; do not claim NFQWS MODULE DONE.

## NFQWS Strategy Library — supersedes older legacy owner description (11.11.2026)
- `NfqwsStrategyLibrary.svelte` + `NfqwsStrategyLibraryModel.js`: Strategy Library persistent candidate editor, list, Save/Delete, Sandbox handoff; direct legacy DOM editor/handlers removed.
- `NfqwsRuntime.js`: only API transport/cache + Sandbox bridge for this screen. Separate Sandbox/Selector and many other NFQWS pages remain imperative. **No full NFQWS MODULE DONE.**

## NFQWS Selector V2 + Auto Setup — new native UI owners (11.11.2026)
- `NfqwsSelector.svelte` and `NfqwsSelectorModel.js`: all Selector controls, live progress, bind, preview, confirmation and application. No legacy `#v2Selector*` DOM writes or event assignments.
- `NfqwsAutoSetup.svelte` and `NfqwsAutoSetupModel.js`: source selection, diagnosis, short-circuit gates, TCP16 branch, timeline, exact preview and Safe Apply; uses registered Svelte Selector controller, no legacy `#v2AutoSetup*` DOM writes.
- `NfqwsRuntime.js` continues other legacy UI/workflow areas (DPI, Domain/Test Lab, catalog/blobs, observed/target memory, history). NFQWS MODULE DONE **PENDING**.
