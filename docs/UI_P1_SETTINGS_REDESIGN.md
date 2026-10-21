# RouterForge UI — P1 Settings redesign

Baseline: `77e025c57486d28ff0cc9736883a847ec16cf7ea` (P0 PASS).

## Design intent

Follow the NFQWS Manager palette/blocks and the Home page typography, using
existing P0 primitives and global `--rf-*` tokens. No new theme or network APIs.

## Information architecture

Four compact, accessible tabs without scroll-heavy sidebar:

1. **System** (default): status metrics, installation accounting and optional
   telemetry; existing DNS/monitoring thresholds remain here if available.
2. **Interface**: general UI settings and complete appearance customization.
3. **Logs**: storage, current path, rotation and save control.
4. **Security**: authentication state, session information and backend.

The seven preexisting settings sections are transplanted verbatim; all handler
expressions, bound values, API calls and authorization checks stay unchanged.
The Core system summary becomes a full-width responsive metric grid, eliminating
the blank sidebar. Installation statistics are nested inside that same system
block rather than inserted as a standalone page-level panel.

## Strict scope

- `components/core/frontend/src/routes/settings/+page.svelte`
- `components/core/frontend/src/routerforge-settings-p1.css`
- `docs/UI_P1_SETTINGS_REDESIGN.md`

No Core Go, module backend, package lifecycle, VPN, NFQUEUE, VPS, firewall,
telemetry payload or release scripts change. This is a frontend-only patch.

## Acceptance gates

- Exact preimage settings Git blob and P0 base SHA.
- Seven section bodies and existing critical event handlers preserved.
- Telemetry in System; local temperature monitoring still available.
- Desktop full-width system grid, compact secondary sections, responsive mobile.
- Svelte typecheck, frontend build, Go tests and rolling Dev publish CI PASS.
- `main`, Stable, Beta and Beta 9 unchanged.
- No router installation or runtime restart as part of P1.

## Follow-up

P2 standardizes the other Core routes, followed by separate per-manager CSS
passes. Visual acceptance also requires screenshots on Dev before promotion.
