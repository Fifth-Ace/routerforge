# NFQWS remaining UI owners — functional source checkpoint / 12.11.2026

## Границы и точный статус

**SOURCE/COMMAND regression: PASS; MODULE DONE: PENDING.** Исходное дерево — единственный актуальный `repository/`, без overlay и без remote commit. По отношению к приложенному handoff выполнен существенный перенос NFQWS UI; текущий Linux Node **229/229 PASS**, owner graph **70/70 PASS**, inventory **11/11 PASS**. Реальный Vite **после этих изменений** и Chromium CDP **не выполнялись**: `npm ci --offline` дал `ENOTCACHED` для `zimmerframe@1.1.5`. Предыдущие Windows Vite 164/126 modules — исторический PASS для исходного handoff, **не для этого checkpoint**.

## Владелец экрана → API / guard / результат

| UI | Native Svelte owner | Действующая команда/API | Защита/проверка | Текущий gate |
|---|---|---|---|---|
| Catalog | `NfqwsCatalog.svelte` | `/configs`, `/upstream/nfqws-menu`, pinned upstream source; Sandbox, Smart Apply handoff | SHA/точная ветка, no auto-apply, selection epochs | Node PASS; browser PENDING |
| Backups | `NfqwsBackupCenter.svelte` | `/backups`, `/backups/read`, `/backups/prune`, `/backups/restore`, `/backups/delete` | explicit confirmation for destructive actions, persistent snapshot, server ownership | Node/source PASS; browser/server PENDING |
| Blobs | `NfqwsBlobs.svelte` | `/blobs`, `/blobs/install`, `/blobs/delete`, `/clienthello/validate` | manifest exact SHA/ref/path, content type, confirm token, GET postverify, no restart | executable 403/409/invalid MIME + source tests PASS; browser PENDING |
| Smart Apply | `NfqwsSmartApply.svelte` | `/config`, prepared local/upstream config | explicit confirmation, CAS/current SHA, backup/recovery guard, rollback | Node/source PASS; browser/server PENDING |
| History / auto-pick / recovery | `NfqwsRunHistory.svelte` | local history, read-only scoring, `/check` only on explicit guard | measured-only eligible run, percent bounds, target cap, confirm | Node negative PASS; browser PENDING |
| Keenetic / health | `NfqwsKeenetic.svelte`, `NfqwsRuntimeHealth.svelte` | `/keenetic`, `/v2/nfqueue-health`, bench profiles, automation gate | GET only, abort on tab switch/unmount, no hidden action | Node/source PASS; browser PENDING |
| Batch domain check | `NfqwsDomainBatchCheck.svelte` | `/check` explicit POST per target | concurrency ≤4, AbortController Stop/Close/unmount | Node negative PASS; browser PENDING |
| Setup feedback, diff, chips | `NfqwsSetupChips.svelte`, `NfqwsSetupDiagnostics.svelte`, `NfqwsSetupDiff.svelte` | existing guarded `/config` command | dirty-state, two-step preview, confirm/CAS | Node PASS; browser PENDING |
| Overview task navigation | `NFQWSManagerApp.svelte` | Svelte navigation callbacks only | no service mutation, no duplicate onclick | Node/source PASS; browser PENDING |
| Setup section accordion | Svelte `use:setupAccordion` from `NfqwsUIActions.js` | local UI only | destroy removes listeners/timers; retained CSS markup and keyboard navigation | Node/source PASS; browser PENDING |
| List/Script workspace | Svelte `use:workspaceHeight` | localStorage workspace height | allowed range 420–900, observer cleanup | Node/source PASS; browser PENDING |
| Service status/backup count | `NFQWSManagerApp.svelte` from `onStatus` | `/status` GET | fixed stale `#backupCount` dereference, no POST on mount; actual mocked boot | executable Node PASS; browser PENDING |
| Geo assets | current UI **placeholder** only | dormant legacy `/v2/geo/*` functions still inside runtime | no mounted controls, not accepted; must migrate/retire safely | **PENDING** |
| Theme / height report / keyboard | legacy runtime retains limited root style, shell reporting and Ctrl+S | local UI only | teardown and single-owner audit incomplete | **PENDING** |

## Owner inventory and removals

- Original handoff runtime (1640 lines): 40 `innerHTML=` and 43 `.onclick=` indicated in original handoff docs. Current checkpoint has **no `.onclick=` assignments** in `NfqwsRuntime.js`; remaining `.innerHTML=` are dormant Geo (four sites). Do not call a simple grep full browser proof.
- Removed inactive legacy syntax editor DOM wrappers, obsolete duplicate resource listeners, old setup collapsible handlers, runtime task onclick assignments and runtime resizable workspace observers. The Svelte-mounted actions own lifetime, markup remains visually compatible.
- Existing command paths remain in command layer with explicit user confirmation; `No surprise POST on mount`, `403/409`, invalid blob MIME and unmount verified in executable `NfqwsRuntimeBoot.test.js`.
- No setup/Geo service mutation occurred while testing. No actual Keenetic/network/router invocation.

## Required next acceptance

1. Run new **pinned NFQWS and Lite Vite** with zero Svelte diagnostics, then `repository/qa-nfqws/browser-owner-gate.mjs` on actual compiled Svelte in Chromium. Local fixture only; exact negative `403/409` and no POST-on-mount. The single PowerShell 5.1 runner in this archive now includes these gates fail-fast.
2. Remove/transfer dormant Geo DOM writers and root theme/height layout ownership; full NFQWS feature matrix browser/API/rollback signoff required before `NFQWS MODULE DONE`.
3. Continue Lite controller, Runtime production RT-C4 Linux Go/IPK, and other eight frontends G0–G8. GitHub CI or router changes require separate explicit authorisation.

**Provenance:** parent ZIP is `/mnt/data/RouterForge_ALL11_FULL_HANDOFF_20261112(1).zip` in this isolated environment; immutable remote BASE SHA `d069c1f09f95392368ab0e128857bc5a67e96c75` is historical pinned expectation, not a new read-only online check. Worktree is unpublished local source.

## Follow-up: Vite accepted on user Windows, browser gate failed

The user now ran real pinned Windows builds for this checkpoint: 177-module NFQWS bundle SHA256 `72f538720798efdad5fbc8ada53b1cd73ea9d5900d4f357604f00febcee52fde`, Lite 126-module SHA256 `fe31587add6373ea4019c81b2c1dc631178f7c944c5bc2d40f270c00ecba66a5`; both 0 Svelte warnings. The same runner **FAILED** at Chromium root wait `REAL_SVELTE_ROOT` before any assertions. New controlled Linux Chromium synthetic-root reproduction proved managed policy can cause this exact symptom (`ERR_BLOCKED_BY_ADMINISTRATOR`); it **does not establish** the user's Windows cause. The new test harness safely supports in-memory browser fixture fallback with exact compiled bundle and explicit error diagnostics. New full app browser gate still PENDING; do not modify runtime business commands based on this unclassified old timeout. See handoff `docs/13_BROWSER_CDP_LOOPBACK_POLICY_AND_RECHECK.md`.
