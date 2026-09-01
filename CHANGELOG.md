# Changelog

All notable public changes to DNS Monitor are documented here.

## [Unreleased]

### Added

- MIT license.
- Entware/opkg repository bootstrap installer.
- Native `dns-monitor` opkg package layout and feed build scripts.
- GitHub Actions CI for tests, vet and shell syntax checks.
- Security policy and GitHub issue / pull request templates.
- Polished Russian-first README with English translation and quick installation instructions.

## [0.1.0] - 2026-09-02

First public release.

### Added

- Automatic Keenetic DoT/DoH resolver discovery.
- Passive DNS request/response observation.
- Resolver health and rolling quality metrics.
- Fallback, timeout and DNS-error tracking.
- Per-device and per-interface DNS attribution using Keenetic client metadata.
- Per-client DNS drill-down with pauseable live flow and filters.
- Client-side `FORWARDED`, `CACHE_LOCAL`, `ERROR` and `CLIENT_TIMEOUT` outcomes.
- Keenetic policy routing discovery and tunnel-path display.
- Policy-aware DoT/DoH diagnostics using `SO_MARK` with default-route comparison.
- Embedded web UI on port 2233.
- ARM64 Entware installer and init script.
