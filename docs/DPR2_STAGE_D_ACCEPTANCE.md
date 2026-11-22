# RouterForge R7 · DPR2-D · Encrypted transports, explicit fallback, diagnostics

**Base:** `feature/dns-router-r7-stage-c` at `105b0c0db4bbaa326c3f1a2766ca848220bd55f5`

**Branch:** `feature/dns-router-r7-stage-d` (isolated; only after guarded delivery).

**Scope:** optional `routerforge-dns-router` experimental runtime, no public channel changes.

## Implemented

1. Four resolver transport types: `udp`, `tcp`, `dot` (RFC 7858 DNS-over-TLS) and `doh` (RFC 8484 POST wireformat).
2. DoT uses TCP DNS framing over TLS; mandatory SNI and verified certificate chain, minimum TLS 1.2; no `InsecureSkipVerify` or user-configurable trust bypass.
3. DoH uses HTTP POST `application/dns-message`, valid TLS, a pinned IP:port connection, fixed HTTPS hostname for SNI/certification, no redirects, no HTTP proxy, strict MIME/status/body-size checks.
4. Encrypted transports require explicit `server_name`, and DoH supports `path` (`/dns-query` by default). `address` remains a literal non-loopback unicast `IP:port` to avoid DNS bootstrap loops.
5. Explicit opt-in fallbacks per primary upstream (`fallbacks` IDs, maximum 3); cross-reference validation, no cycles/nested fallback, no use of arbitrary system or router DNS. Legitimate DNS error responses such as NXDOMAIN or SERVFAIL are passed through, not retried against other providers.
6. Passive `GET /api/modules/dns-router/upstreams/status`, preserving Unix-socket Core proxy; in-memory per-upstream attempts, success, error and last latency; no query names logged. No polling or active probing.
7. UI updated with transport, SNI, DoH path and explicit reserve pool fields. Package example and CI verification updated.
8. Experimental build matrix `aarch64-3.10`, `mips-3.4`, `mipsel-3.4`; no auto-start scripts or public release modification.

## Acceptance gates

- `gofmt -l` returns no files in the delivered payload.
- `go test -race ./modules/dns-router-runtime/...` and `go vet ./modules/dns-router-runtime/...` are green.
- `TestDPR2D*` covers TLS verification, HTTPS protocol rejection, explicit failover, local DNS leak prevention, and passive diagnostics.
- `sh scripts/verify-dns-router-experimental.sh` produces 3 standalone IPK with `Depends: routerforge-core`, no DNS Manager dependency.
- Core/R7-A/B/C behavior is kept stable; protected branches and releases unchanged.

## Explicitly out of scope

- No binding of port 53, no `opkg dns-override`, no iptables/nft redirect, no DNS interception.
- No persistence of active forwarding/auto-start, no automatic reload of saved rules into a running forwarder.
- No DNS response cache, blocklists, ad-filtering, DHCP, query history or WAN/VPN route manipulation.
- No runtime feature published to Stable, Beta, Dev or App Center.

## Next large step: DPR2-E

- Runtime lifecycle, stale config detection/hot-reload with rollback, benchmark RAM/RSS and latency, router-side read-only discovery of Keenetic DNS Proxy capabilities, dry-run activation plan and clean recovery; **hardware test first on DSTest with operator consent, not production**.
- Strictly separate DNS-upstream selection from Keenetic PolicyN/VPN routing.
