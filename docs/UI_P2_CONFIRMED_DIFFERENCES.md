# RouterForge UI P2 — audit of confirmed differences

Audit date: 2026-10-22. Exact source reference: `2cee58d32049c8fe831473c8faf7f74fbaaa4eed` (`dev`).

## Governing principle

**KEEP is success.** Only change an observable or source-provable discrepancy. Do not homogenize every widget or rewrite a functioning workflow. NFQWS Manager establishes panel/status palette; the Home page establishes readable typography.

## Audited Core surfaces

| Surface | Source evidence | Decision |
|---|---|---|
| Home | Native Core page; current typography is the chosen reference | KEEP, 0 changes |
| Monitoring | `MonitoringModuleApp.svelte`, `main.js`, `module.css`; inherits Core `app.css`, `admin-shell.css`, `routerforge-shell.css`; uses `--rf-*` tokens | KEEP, 0 changes |
| DNS | Standalone frame; canonical tab strip near end of `module.css` supersedes legacy `#38f47b` rules, active uses `--rf-accent`; panel and type rules already present | KEEP, 0 changes |
| Control / Admin | `AdminModuleApp.svelte` and `module.css`; Core fonts/theme loaded, service controls use semantic `--rf-*` | KEEP, 0 changes |
| Network Tools | CSS maps `--bg`, `--accent`, `--panel`, etc. to `--rf-*` tokens; custom tab state maps to Core accent | KEEP, 0 changes |
| Port Access | `module.css` uses `--rf-*`, and runtime syncs host theme | KEEP pending runtime visual verification |
| CrowdSec | `app.js` actively synchronizes host theme tokens to `--cs-*`; hardcoded CSS values are fallbacks rather than proof of runtime mismatch | KEEP pending runtime visual verification |
| NFQWS Manager | User-selected visual reference | KEEP |
| Antiscan | Six binary settings in `frontend/index.html` + cron scheduler enabled switch in `frontend/app.js` use native checkbox glyphs, conflicting with P1A canonical ON/OFF + track | **FIX: seven binary controls only** |

## Exact Antiscan scope

Native checkboxes remain actual form controls, preserving `.checked`, `.disabled`, DOM IDs, query selectors, serialization, save and reload behavior. Add only status `ON/OFF` and a slider track as visual siblings. CSS uses existing semantic colors; focus and disabled states remain meaningful.

Six config fields: `cfgEnableIpsBan`, `cfgEnableHoneypot`, `cfgUseCustomExclude`, `cfgSaveIPSets`, `cfgSaveOnExit`, `cfgReadNDM`. Seventh is the per-task `data-scheduler-enabled` checkbox in the scheduler row template. No changes to cron expression inputs or `automatic` disabling.

### Accepted file paths only

- `modules/antiscan-manager/frontend/index.html`
- `modules/antiscan-manager/frontend/app.js`
- `modules/antiscan-manager/frontend/module.css`
- `docs/UI_P2_CONFIRMED_DIFFERENCES.md` (this report)

### Safety / regressions

- Git blob preimages checked against the exact Dev base, no loose patch matching.
- Exactly six static binary controls, one scheduler template; no other markup changed.
- No API, backend, firewall, module lifecycle, service or router runtime changes.
- Verify `git diff --check`, exact four-file scope, CI `test`, rolling ARM64 Dev publish and exact Antiscan package asset; Main / Stable / Beta unchanged.
- **Hardware visual verification remains pending** after Dev module update: keyboard, mouse, busy/disabled, tablet width, theme and actual save/reload. CI alone cannot prove appearance.

## Next audit

Review individual NFQWS / Port Access / CrowdSec / Antiscan screens on DSTest only when there is real evidence (screenshots or a clear source discrepancy). Keep otherwise; no mandatory new commits.
