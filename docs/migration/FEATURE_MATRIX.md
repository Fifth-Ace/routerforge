# Feature matrix / 11 native Svelte owners

| Module | Source/entry | Actual owner state | Acceptance |
|---|---|---|---|
| Core | PASS | native Svelte | behavioral audit PENDING |
| Admin | PASS | native Svelte | behavioral audit PENDING |
| Monitoring | PASS | native Svelte | behavioral audit PENDING |
| DNS | PASS | native Svelte | behavioral audit PENDING |
| CrowdSec Manager | PASS | native Svelte | behavioral audit PENDING |
| Network Tools | PASS | native Svelte | behavioral audit PENDING |
| Antiscan Manager | PASS | native Svelte | behavioral audit PENDING |
| Port Access Manager | PASS | native Svelte | behavioral audit PENDING |
| DNS Router Runtime | PASS | Svelte state + commands; RT-C3 34/34 browser PASS | RT-C4 production Go/IPK/upgrade PENDING |
| DNS Router Lite | PASS | Svelte owns P25 forms, DNS editor, logs, shell; active command controller remains | L07/L08 browser/Go/IPK PENDING |
| NFQWS Manager | PASS | Svelte owns nav/logs/Lua/DPI/overview/backup/lists/Smart Apply + setup form fields/modes/custom strategy editor; legacy raw config/library/list editors still active | N03–N09 PENDING |

PASS for source/entry is not proof of module ownership, no static counter for signoff. No backend/router changes in this slice.

## NFQWS / active raw nfqws2.conf — 11.11.2026
| Feature | UI owner | Status |
|---|---|---|
| Raw editor, dirty state, diff, Ctrl+S, fullscreen | NfqwsRawConfig.svelte | SOURCE_TRANSFERRED / WINDOWS_BUILD_PENDING |
| Syntax validation + native dry-run output | NfqwsRawConfig.svelte | SOURCE_TRANSFERRED / BROWSER_PENDING |
| Lua catalog and selected argument payload fix | NfqwsRawConfig.svelte | SOURCE_TRANSFERRED / BROWSER_PENDING |
| Save confirm, preflight SHA, backend backup/reload | Svelte → existing backend | CLIENT_GUARDS_PASS / ATOMIC_SERVER_CAS_PENDING |
| Hostlist/ipset/List Editor | NfqwsRuntime.js | ACTIVE_LEGACY / TODO |

## NFQWS List Editor — 2026-11-11
| Feature | UI owner | Acceptance |
|---|---|---|
| List file inventory, search, syntax, draft, normalize, CRUD, transactional rename | `NfqwsListEditor.svelte` | NODE 98/98 PASS, Windows Vite PENDING |
| Entry inspector, static path, cross-list analysis | `NfqwsRuntime.js` legacy read-only/mutating analytics bridge | SVELTE TRANSFER PENDING |
| Strategy Library, remaining NFQWS panels | legacy DOM | PENDING |

## 11.11.2026 — NFQWS native Strategy Library
- New: inventory, CRUD draft, explicit Save/Delete, cross-navigation to Sandbox, error/dirty state owned by `NfqwsStrategyLibrary.svelte`. Backend route/token parity validated against `strategy_library_v2.go`.
- Runtime: no Strategy Library DOM renderers/onclick; cache remains available to Selector/Sandbox.
- PENDING: pinned Vite + real browser; Strategy Library server-side atomic CAS; independent Selector/Sandbox Svelte port.
