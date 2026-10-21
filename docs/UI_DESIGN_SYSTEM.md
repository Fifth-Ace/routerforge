# RouterForge UI Design Contract — v1 (P0 Foundation)

**Baseline:** `dev@4e966c2dc658639d8fe2f07a54de544ef72e78b4`.

## Visual precedents

- **NFQWS Manager:** palettes, section and metric cards, borders, tabs, toolbars, state indicators, density.
- **RouterForge Home:** readable typography, headings, information hierarchy, concise labels.
- **RouterForge Settings:** first complete redesign target, telemetry integrated in **System**.

The existing `routerforge-shell.css` is the authoritative root of semantic `--rf-*` tokens. Do not replace or fork its theme. `routerforge-ui-primitives.css` is additive and contains ONLY `.rfu-*` classes and `--rfu-*` variables. P0 classes are unused until the page-by-page migrations: importing this stylesheet must not change current visuals.

## Palette and rules

| Semantic role | Source / default | Rule |
|---|---|---|
| Background | `--rf-bg` (`#0b0d10`) | Root app background |
| Surface | `--rf-surface` (`#12151a`) | Cards, panels |
| Surface 2 | `--rf-surface-2` (`#171b21`) | Controls, secondary areas |
| Border | `--rf-border` (`#29313a`) | Basic 1px outline |
| Strong border | `--rf-border-strong` (`#36414d`) | Focused, destructive, selected surfaces |
| Text | `--rf-text` (`#f5f7fa`) | Headings and body |
| Muted | `--rf-muted` (`#8d98a4`) | Descriptions and metadata |
| Accent | `--rf-accent` (`#38bdf8`) | Interactive focus/selection, theme-aware |
| Success | `--good` (`#2ea043`) | Running, pass, online |
| Warning | `--warn` (`#d29922`) | Updates and warnings |
| Error | `--bad` (`#f85149`) | Stopped/error/fail |

Never use green for a generic primary action. Never conflate `UPDATE` with `STOPPED` or `READY` with `RUNNING`.

## Typography, spacing, responsive

- Body: `Inter` / `Segoe UI Variable Text` / `Segoe UI` / `sans-serif`, 13–14px, 1.45 line-height.
- Title: use Home hierarchy, usually 23–26px; H2 16–19px; card titles 13–15px.
- Monospace: technical IDs, versions, SHA, terminal paths, logs only.
- Spacing: 4 / 8 / 12 / 16 / 20 / 24px. Typical card gap 12px; action minimum height 34px; radii cards ~8px.
- Supported: 320/390/768/1024/1440/1920px, 100/125/150% zoom, RU/EN, density compact/default/comfortable.
- Focus must remain visible, keyboard navigation must work, and `prefers-reduced-motion` must be honored.

## Primitive classes supplied by P0

| Class | Purpose |
|---|---|
| `.rfu-page-head` / `.rfu-page-heading` | Page heading, title/subtitle, optional actions |
| `.rfu-section` / `.rfu-section-head` | Large content section with bounded padding |
| `.rfu-metric-grid` / `.rfu-metric-card` | Grid for figures, responsive 4→2→1 |
| `.rfu-action-grid` / `.rfu-action-card` | Task-oriented quick actions |
| `.rfu-button`, `.rfu-button--primary`, `.rfu-button--danger` | Consistent buttons |
| `.rfu-tabs`, `.rfu-tab[aria-selected]` | Accessible tab nav appearance |
| `.rfu-status[data-state]` | State chips, categorical status |
| `.rfu-setting-row` | Setting label/helper/control layout |
| `.rfu-empty`, `.rfu-notice` | Empty and notification states |

The classes are opt-in. No global selectors for `body`, `button`, `input`, `table` or module-specific selectors. Module CSS should map its existing local variables (`--pa-*`, `--cs-*`) to Core semantic tokens with safe fallbacks, rather than importing Core's global layout CSS into independent HTML pages.

## Settings IA (P1)

Tabs: `System` (default) → `Interface` → `Logs` → `Security`. In System, compose a compact status summary and embedded RouterForge statistics: random installation ID accounting, optional module/version diagnostics, collector reachability and registration status. No CPU/RAM/temperature is transmitted; local CPU temperature warning remains under Interface. Keep API requests, server handlers, authorization and storage semantics unchanged.

## Definition of Done

Every touched page gets a before/after screenshot, a layout and keyboard pass, a semantic state pass, RU/EN and theme pass, and functional verification. Scope is Dev-only; no Stable/Beta/VPS/router changes. For project-wide acceptance, see `docs/UI_UNIFICATION_ROADMAP.md`.
