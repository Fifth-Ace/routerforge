# RouterForge changelog

RouterForge components are versioned independently. Entries below describe platform milestones; they are not a promise that every package shares the same version.

## [Unreleased]

- Canonical GitHub repository renamed to `Fifth-Ace/routerforge`; legacy links remain supported through redirects and the Core compatibility bridge.

## 2026-09-03 — RouterForge 0.3 generation

### Added

- RouterForge product identity and unified router-console UI.
- Capability-driven navigation: Home, Monitoring, DNS, Control, Marketplace and Settings.
- Official package namespace:
  - `routerforge-core`
  - `routerforge-dns`
  - `routerforge-admin`
  - `routerforge-system`
  - `routerforge-thermal`
  - `routerforge-storage`
  - `routerforge-network`
  - `routerforge-profiling`
- Independent component versions and per-channel release indexes.
- Rolling `routerforge-beta` and `routerforge-stable` GitHub release channels.
- SHA256-verified RouterForge package lifecycle from Marketplace.
- Web update for Core and optional modules.
- Batch RouterForge update with Core updated last.
- Hourly automatic remote update checks plus synchronous manual refresh.
- Marketplace update-count highlighting.
- Optional Entware-root authentication with 12-hour in-memory sessions.
- RouterForge Control read-only helper.
- System, Thermal, Storage and Network monitoring providers.
- Profiling capability bound to loopback.
- RouterForge design system, typography normalization and platform dashboard.

### Changed

- Project evolved from a DNS-only application into a modular router platform.
- DNS became an independently managed capability while the DNS engine remains inside Core.
- Stable Core uses Stable release/Registry sources; Beta Core uses Beta/dev sources.
- Release publishing preserves unchanged component assets instead of replacing a same-version binary.

## [0.1.0] - 2026-09-02 — legacy DNS Monitor

First public DNS Monitor release.

### Added

- Keenetic DoT/DoH resolver discovery.
- Passive DNS request/response observation.
- Resolver health and rolling quality metrics.
- Fallback, timeout and DNS-error tracking.
- Per-device and per-interface DNS attribution.
- Per-client DNS drill-down.
- `FORWARDED`, `CACHE_LOCAL`, `ERROR` and `CLIENT_TIMEOUT`.
- Keenetic policy-routing discovery and route-aware diagnostics.
- Embedded web UI on port 2233.
- ARM64 Entware packaging.

The canonical repository is now `Fifth-Ace/routerforge`. Legacy `dns-monitor-*` packages, paths and historical links remain supported only for migration compatibility.
