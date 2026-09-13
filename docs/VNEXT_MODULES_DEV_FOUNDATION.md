# RouterForge vNext modules — Dev foundation

This document records the first production-shaped implementation of the four top-level modules that were visual-only in Concept R1.

## Scope

The foundation adds four optional Module ABI v1 packages:

- `routerforge-maintenance` — Maintenance / Config Vault inventory / Storage Doctor foundation.
- `routerforge-network-tools` — Network Doctor / Route Inspector / metadata-only Flow Explorer.
- `routerforge-integrations` — installed-only third-party discovery and NFQWS2 Manager diagnostics foundation.
- `routerforge-developer-tools` — runtime diagnostics and Module ABI manifest validation.

The module UIs follow the dense dark RouterForge Concept R1 workspace: compact metrics, evidence tables, explicit PASS/WARN/FAIL semantics, bounded data and no decorative marketing surfaces.

## Shared platform layer

`internal/platform` introduces minimal, reusable contracts for:

- Policy Objects.
- Snapshot/Transaction state transitions.
- Probe Engine DNS/TCP primitives.
- Event Engine bounded ring storage.

These are source-level shared contracts, not standalone daemons or mandatory user packages.

## Safety boundary

This stage is deliberately read-only. Core registers the new modules in `moduleSockets` and `modulePackageNames`, while the existing generic module proxy keeps unknown/non-DNS/non-Admin module IDs restricted to GET/HEAD.

No new lifecycle, config mutation, restart, file-write, NFQWS2 strategy selection or automatic remediation endpoint is exposed. Later mutation work must pass the shared Snapshot/Transaction consumer gate and prove rollback under injected failure before Beta.

## Footprint boundary

All four optional packages compile from one compact stdlib-only runtime source. Each package starts only when installed. The UI is static HTML/CSS/JS with no additional frontend runtime dependency. Flow Explorer reads bounded conntrack metadata and does not capture payloads or add DPI.

## CI gate

`.github/workflows/vnext-modules.yml` performs formatting, tests, vet, ARM64 cross-build, static UI syntax checks, shell validation, Unix-socket smoke tests, builds four ARM64 IPKs and uploads exact-SHA artifacts. It does not publish Beta/Stable channels.
