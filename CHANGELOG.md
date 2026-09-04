# RouterForge changelog

RouterForge components are versioned independently. Entries below describe platform milestones; they are not a promise that every package shares the same version.

## [Unreleased]

- DNS Control foundation for Core `0.3.7-beta`:
  - `Tools → DNS details` now reads the full Keenetic `show dns-proxy` state: upstream configuration, per-policy request/cache statistics, server rank/latency counters, static A/AAAA records and rebind protection.
  - Identical resolver endpoints are presented as one logical upstream with all discovered domain bindings while the per-policy view preserves each physical ndnproxy instance.
  - DNS details can be refreshed, copied to the clipboard and exported as a text report; this first step is intentionally read-only before DNS write/rollback support lands.
- Canonical GitHub repository renamed to `Fifth-Ace/routerforge`; legacy links remain supported through redirects and the Core compatibility bridge.
- Monitoring cleanup for the next Beta:
  - Thermal collapses mirrored `thermal_zone` / `hwmon` sensors and adds human-readable MT7988 roles.
  - Storage hides internal flash block devices and duplicate mount aliases from the normal view while keeping raw data in Advanced mode.
  - Network uses active KeeneticOS logical interfaces, maps them to Linux `system-name` for counters, and keeps all kernel interfaces in Advanced mode.
  - Network interface and IPv4 route tables support click-to-sort columns with persistent direction during live refreshes.
- RU/EN localization is complete across the RouterForge frontend:
  - Home, Control, Marketplace, Settings, authentication, dialogs, redirects and every DNS subpage use the shared i18n dictionaries.
  - Browser titles, confirmations, empty states, alerts, tooltips/ARIA labels and frontend-rendered status text switch live with the selected language.
  - Locale-aware time, duration, relative-time and frontend status formatting now follows the current UI language.
  - Russian remains the default and fallback locale; technical IDs and user/Keenetic-provided data remain unchanged.

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
