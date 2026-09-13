# RouterForge UI Concept R1

Status: **visual implementation scaffold / mock data / no mutations**
Branch: `concept`
Base: exact successful `dev` commit captured at concept creation time.

## Purpose

This branch is a safe visual proving ground for the RouterForge vNext module layout described by the current Master Development Plan. It intentionally does **not** replace production module APIs, does not add privileged mutations, and does not change Stable/Beta/Dev publication channels.

The test route is:

```text
/concept
```

The page runs inside the existing authenticated RouterForge shell and gives one switchable mock screen for every major user-facing module family:

1. App Center
2. Monitoring
3. DNS
4. Management
5. Maintenance
6. Network Tools
7. Integrations
8. Developer Tools

## Product mapping

| Concept screen | Planned / current capability shown |
|---|---|
| App Center | Registry, sources, neutral technical catalog, RouterForge modules, integrations, Entware |
| Monitoring | System/Thermal/Storage/Network, Health & Alerts, Incident Timeline |
| DNS | Resolver workspace plus DNS Policy Router direction |
| Management | Processes, Services, Packages/Ports, File Manager, Terminal, NDM Console, Service Inspector |
| Maintenance | Logs, Config Vault, Backup/Recovery, Tasks/Cron, Watchdogs, Support Bundle, Storage Doctor |
| Network Tools | Network Doctor, Route Inspector, Flow Explorer, Active Probes |
| Integrations | discovered Web UIs, third-party management, NFQWS2 Manager direction |
| Developer Tools | Profiling, raw logs/events, manifest validation, API explorer, feature flags |

## Safety boundary

- All visible data on `/concept` is mock data.
- Buttons that resemble mutations only show a concept notification; they do not call RouterForge APIs.
- No existing module route is replaced.
- No existing backend or Unix-socket contract is modified.
- No `main`, Beta, Stable or immutable release reference is touched by this branch.
- The branch-specific GitHub workflow builds an ARM64 `routerforge-core` test artifact only; it does not publish any RouterForge channel or move rolling tags.

## Test-router intent

The concept artifact is intended only for the dedicated development/test router. The concept package replaces the Core package for visual evaluation and is expected to be rolled back to the normal Dev package after review.

The UI route should be tested primarily for:

- information density;
- responsive layout;
- module navigation mental model;
- reusable panel/table/card patterns;
- consistency with the current RouterForge dark shell;
- whether planned features fit naturally before backend implementation begins.

## Promotion rule

`concept` is not a release branch. Accepted pieces should later be reworked or cherry-picked into `dev` in small, reviewable feature commits with real APIs, targeted tests, hardware evidence and normal RouterForge safety gates.
