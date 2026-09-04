# RouterForge changelog

RouterForge components are versioned independently. Entries below describe platform milestones; they are not a promise that every package shares the same version.

## [Unreleased]

- DNS `0.4.1-beta` accepts both Keenetic RCI shapes for saved plain DNS (`[]` and `{"server":[...]}`), fixing resolver discovery on Hopper when no static plain DNS servers are configured.
  - DNS data-load errors no longer claim that the module itself is unavailable when its health endpoint is online.
- Core `0.4.1-beta` fixes the Module ABI v1 UI proxy: module directory URLs now preserve their trailing slash, so a module UI cannot escape from `/api/modules/<id>/ui/` into the Core SPA and render a nested RouterForge shell with a `404`.
  - `routerforge-dns` remains `0.4.0-beta`; this is a Core module-host fix, not a DNS module change.
- RouterForge `0.4.0-beta` introduces Module ABI v1 and turns DNS into a real independently versioned module:
  - Core keeps the web shell, auth, Marketplace/Registry and the generic module API/UI host; DNS capture, discovery, health, diagnostics and mutation logic move to the `routerforge-dns` runtime over a root-owned Unix socket.
  - `routerforge-dns` now ships `/opt/bin/routerforge-dns`, `S91routerforge-dns`, its own UI bundle and `/api/modules/dns/*`; future DNS-only changes no longer require rebuilding or version-bumping Core.
  - Legacy `/api/snapshot`, `/api/history`, `/api/plain-dns` and `/api/dns/info` remain compatibility bridges through Core during the 0.4 migration.
  - DNS Control adds resolver Add/Edit/Delete, temporary Disable/Enable, native multi-domain grouping, dynamic DHCP/service DNS read-only protection, structured Keenetic RCI writes, readback verification and rollback.
  - Resolver presentation uses RouterForge terminology: Overview, Resolvers, Rules, Traffic and Diagnostics. Physical ndnproxy entries remain available as advanced details instead of defining the normal UI.
  - Live Hopper findings are handled: same-address DoT resolvers remain distinct when SNI differs; Yandex `.ru/.su/.рф` entries group into one logical resolver; duplicate static A/AAAA records are collapsed independently of internal flags; policy names are enriched from Keenetic RCI when available.
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
