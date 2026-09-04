# RouterForge changelog

RouterForge components are versioned independently. Entries below describe platform milestones; they are not a promise that every package shares the same version.

## [Unreleased]

- DNS `0.4.13-beta` introduces Resolvers 2.0, combining the original master/detail server UX with the validated Module ABI DNS Control feature set:
  - the default resolver view is a stable master list plus selected-resolver detail pane with CRUD/Disable/Enable, dynamic read-only protection, logical/native metadata and persistent selection across refreshes;
  - logical DNS/DoT/DoH resolvers are correlated with existing runtime telemetry; multi-domain secure resolvers aggregate their native entries into 5m/1h/24h quality, diagnostics and recent-flow views;
  - plain DNS details reuse the passive resolver tracker, while DoT/DoH details restore quality windows, health probe stages, runtime ports and discovery metadata;
  - an optional Cards view preserves the newer DNS Control presentation with equalized card geometry, bounded endpoints and a direct Details action back into the master/detail view;
  - search/protocol/status filters, 8-slot secure preflight, 16-domain plain-DNS guard and snapshot/readback/rollback mutation semantics are unchanged.
- DNS `0.4.12-beta` polishes the restored Module ABI DNS layout without changing resolver control:
  - the same-origin DNS iframe now follows the module's real content height (with a viewport floor), eliminating the unnecessary nested vertical scrollbar and letting the Core page/browser own scrolling;
  - iframe height follows tab/content changes via ResizeObserver and parent/window resize events;
  - the oversized standalone Native mode panel on Resolvers is replaced by a compact readback/rollback safety note while the native multi-entry explanation remains in the resolver panel header.
- DNS `0.4.11-beta` restores the observability/control UI parity that was lost during the Module ABI v1 split without moving DNS back into Core:
  - Overview again shows searchable active/all runtime tables for plain DNS and protected DoT/DoH, including health, latency, errors, fallback, quality and Keenetic policy contexts;
  - Rules keeps the new native domain-binding view and restores observed fallback routes, Keenetic profile summaries, local records and rebind state;
  - Traffic restores history windows, live DNS Flow with pause/filtering, per-device drill-down, interfaces, domains/QTYPEs and plain-DNS recent events;
  - Diagnostics restores error bursts/journal, local proxy connections, runtime DNS health diagnostics and process/system details;
  - the module reuses the existing Module ABI endpoints (history, fallbacks, clients, client, interfaces, system, error-bursts and plain-dns), refreshes view-specific data lazily, and leaves the already validated resolver mutation/rollback backend untouched.

- DNS `0.4.10-beta` fixes DoH logical-ID stability after successful native writes:
  - Keenetic DoH readback exposes the HTTPS endpoint but no separate port field; RouterForge now reconstructs port 443 (or a custom port embedded in the URL) before calculating resolver identity;
  - this keeps the ID returned by `Create` identical to the ID reconstructed by the next `loadState`, so immediate PATCH/Disable/Enable/Delete operations no longer return a false 404;
  - regression tests cover default HTTPS port 443 and a custom URL port.
- DNS `0.4.9-beta` fixes Keenetic DoH native writes after live Hopper readback validation:
  - DoH write payloads now use Keenetic's `url` RCI argument while readback continues to accept the saved/runtime `uri` shape; posting `uri` was transport-valid but NDMS silently discarded the upstream;
  - canonical verification treats `url` and `uri` as the same DoH endpoint, so write/readback semantics match the firmware;
  - documented DoH SPKI support is preserved through normalization, native entry generation, RCI writes and the resolver editor.
- DNS `0.4.8-beta` fixes the DNS overview runtime regression introduced by the secure DoT/DoH capacity rename:
  - the overview cache metric still referenced the removed `dotSlotLimit` variable, which caused the Svelte render to throw after `loadAll()` completed and left the page frozen on the loading ellipsis;
  - the overview now uses the combined `secureSlotsUsed/secureSlotLimit` values, matching the resolver editor and documented shared DoT/DoH capacity.
- DNS `0.4.7-beta` completes documented DoH/plain-DNS parity in DNS Control:
  - Keenetic's documented secure-resolver capacity is treated as one combined pool of 8 physical DoT/DoH entries, so DoH and DoT cannot overcommit each other;
  - DoH domain bindings use the same logical-to-native expansion model as DoT; the editor projects the combined secure slot usage before save;
  - plain `ip name-server` keeps native multi-domain grouping but now enforces Keenetic's documented maximum of 16 domains per DNS server;
  - DoH format is limited to the documented `dnsm`/`json` values, and its effective port is derived from the HTTPS URL instead of a separate UI field;
  - regression tests cover mixed DoT/DoH 8-slot capacity, DoH multi-domain expansion, DoH URL-port semantics and the 16-domain plain-DNS limit.
- DNS `0.4.6-beta` fixes visual alignment inside the Module ABI iframe:
  - removes the erroneous second max-width/centering layer so DNS fills the full Core content column exactly like native pages;
  - copies Core's computed responsive UI tokens into the same-origin DNS iframe, so 2K/4K `auto` scaling matches the shell even though the iframe viewport is narrower after the persistent rail;
  - DNS typography, page padding, controls, tabs, cards, panels and modal now use the live Core scale variables instead of hard-coded 13px-era values.
- DNS `0.4.5-beta` visually aligns the Module ABI UI with the RouterForge Core shell:
  - the DNS iframe now uses the same 1440px centered page canvas, 20px page padding, header rhythm, typography, surfaces, borders and controls as Core monitoring pages;
  - DNS view tabs use the Core subtab language instead of a second pill-navigation system;
  - metric cards, panels, resolver cards, tables, state chips, buttons, notices and form controls now share Core density and spacing;
  - the duplicate resolver-page `+ Add DNS` action is removed; the global page action remains, while DoT slot usage moves into the resolver panel header;
  - the resolver editor is tightened to Core form dimensions and warning styling without changing mutation behavior.
- DNS `0.4.4-beta` adds a preflight guard for the Keenetic DoT capacity confirmed on live Hopper hardware:
  - Hopper accepts exactly 8 native DoT upstream entries; a 9th entry is silently truncated by NDMS.
  - RouterForge now rejects a mutation that would exceed 8 DoT physical entries before any native RCI write, returning a conflict instead of relying on rollback.
  - `/resolvers` exposes current DoT physical usage and the limit; the DNS UI shows `used/8`, projects the post-save usage, and disables Save when the requested logical resolver would exceed capacity.
  - Live validation confirmed 8/8 succeeds, 9/8 triggers readback rollback on `0.4.3-beta`, and cleanup restores the original native DNS snapshot byte-for-byte.
- DNS `0.4.3-beta` fixes Keenetic DoT SNI write/readback after live Hopper mutation testing:
  - structured RCI writes now translate RouterForge SNI to Keenetic's saved `fqdn` field, preventing NDMS from silently dropping SNI and triggering a readback rollback;
  - diagnostics no longer fabricate `SNI=<address>` for native DoT entries that have no SNI, and accept both `sni:` and `fqdn:` metadata labels.
  - the live failed mutation was fully rolled back and the native `/show/sc/dns-proxy` snapshot matched its pre-test SHA256 byte-for-byte.
- DNS `0.4.2-beta` polishes the Module ABI UI:
  - DNS tabs switch in-place without reloading the Core route/iframe, eliminating the visible page jump.
  - The resolver protocol selector uses RouterForge-owned dark styling and a consistent chevron instead of browser-native select chrome.
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
