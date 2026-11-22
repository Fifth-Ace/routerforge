# ADR-R7-009 — Optional DNS Router module (accepted, Stage A)

**Date:** 2026-10-27. **Scope:** RouterForge DNS upstream selection; *not* VPN/WAN routing.

## Decision

`routerforge-dns-router` shall be an **independent optional OPKG package** with no dependency on `routerforge-dns`. It is separate from the existing DNS Manager and requires only RouterForge Core for future UI integration. Installation must not silently change Keenetic `ndnproxy`, DNS port 53, DNS override, iptables, WAN policies, nor existing resolver settings. A plain DNS Monitor user shall not pay the DNS Router disk/RAM cost.

## Existing P18 and important semantic difference

The historical P18 implementation in `modules/dns/runtime/dns_policy*.go` chooses a **Keenetic connection policy (PolicyN / egress path)**, not a DNS upstream pool. The existing runtime contains valuable transactional, DNS-parsing, marked egress and reversible takeover infrastructure. Reuse is approved **only with parity tests**. Copying PolicyN rules into upstream-routing config, or removing existing DNS Manager code in Stage A, is forbidden.

## Stage A implementation (this change)

- Standalone Go package `modules/dns-router-runtime` with a bounded, deterministic, immutable suffix-trie rule matcher.
- Explicit IP:port upstream profiles, UDP/TCP metadata, exact/suffix/subdomains modes, 10,000-rule ceiling, system DNS fallback (no automatic fallback to arbitrary external DNS).
- `.local`, `.lan`, `.home.arpa`, `localhost`, IPv4 and IPv6 reverse namespaces keep Keenetic DNS; unsafe loopback upstreams are rejected.
- Offline evaluator CLI, Go tests, experimental **not-published** three-architecture IPK build and an existing CI gate.
- No DNS socket, service, App Center catalog entry, install scripts, live traffic capture, or router mutation yet. Example configuration is data, not an active configuration.
- The public release manifest remains at **10 components**. No `11th IPK` claim until all release, install, App Center and CI contracts are updated in a later feature gate.

## Stage B (whole feature, not piecemeal patches)

Add the independent Unix-socket API/UI and policy CRUD + atomic persistence + import/export; separate `upstream` from `WAN PolicyN` and define migration/compatibility. Prototype protected Keenetic local-name forwarding. Do not run a privileged or 53-bound listener.

## Stage C

Build/prove guarded UDP/TCP sidecar, loop detection, fail-closed upstream selection, native DoT/DoH adapter bake-off, safe snapshot→apply→verify→rollback, cold boot and restart acceptance. Benchmark Go sidecar against dnsmasq/SmartDNS and report **measured** memory/flash/latency.

## Stage D

App Center integration, independently versioned official IPK and full 11-component × 3-arch release-contract migration, hardware tests, staged Beta, then Stable by explicit approval. Avoid mass version increments.

## Exit gates for Stage A

Exact dev source SHA; feature branch only; preimage-guarded CI change; all Go tests PASS, executable CLI validation PASS, 3 target IPKs built and metadata validated, no user router touched, default branch / Beta / Stable tags unchanged. CI live RUN ID captured directly from GitHub dispatch API response.
