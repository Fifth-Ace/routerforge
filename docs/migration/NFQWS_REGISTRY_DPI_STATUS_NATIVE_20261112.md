# RouterForge ALL11 — Registry import refresh + native DPI status (2026-11-12)

BASE `Fifth-Ace/routerforge` `dev`: `d069c1f09f95392368ab0e128857bc5a67e96c75`.

## Functional fix / Strategy Import → Registry
- Historical gate `NATIVE_REGISTRY_IMPORT_REFRESH_MISSING` required a nonexistent legacy callback inside `NfqwsRuntime.js`; the Svelte import component already called its Svelte prop directly. Restoring a legacy DOM callback would be incorrect.
- `NfqwsStrategyImport.svelte` now additionally validates the exact Go server response (`ok=true`, `source=import`, `production_mutation=false`, integer counts and returned strategy IDs) *before* signaling a successful import.
- After confirmed `/v2/strategies/import` POST, `importedStrategiesChanged()` increments both `registryRefreshSeq` and `strategyLibraryRefreshSeq` in `NFQWSManagerApp.svelte`. `NfqwsStrategyRegistry.svelte` and `NfqwsStrategyLibrary.svelte` fetch their own authoritative inventories from the server on activation or invalidation.
- Unused `onRegistryInvalidate` legacy runtime parameter has been removed. No backend mutations or implicit retry; import remains explicit with confirmation.

## Functional Svelte migration / DPI status
- `NfqwsDPIStatus.svelte` + pure `NfqwsDPIStatusModel.js` own read-only status badges, runtime summary, tab status and Keenetic diagnostics. The legacy module emits a structured snapshot from the existing GET `/dpi-detector` and no longer writes these UI elements by `innerHTML`, `textContent` or class replacement.
- Busy/error/incompatible states are fail-closed: `READY` requires the exact boolean values `installed===true` and `execution_ready===true`, a fresh non-error response and no loading flag. Untrusted fields render as escaped Svelte text.
- **Not yet migrated:** DPI Console/Web forms, live NDJSON renderer, stream control, and Processing Map. Existing API and DPI runner remain unchanged. No router installation or active config mutation.

## Verification / boundaries
- Native Node behavior regression: NFQWS **193/193**, DNS Lite **22/22** (local Linux Node 22). JS syntax PASS.
- Full pinned Windows Vite/Svelte production build **PENDING**; execute `RUN_NATIVE_OWNER_PS51.ps1` from this archive (after SHA256 verification). It must stop on any compiler diagnostics. Browser/Go/IPK/upgrade and ALL11 MODULE DONE **PENDING**.
- Repository `dev` remote and protected refs are not changed. Commit/push/CI dispatch/release/router UNTOUCHED.
