# RouterForge DNS Router R7 — DPR2-C (experimental)

## Delivered as one atomic feature branch

- Parent is exact DPR2-B commit `a7a38e9318da8a6432f015eea514bbeebe6e8981`.
- Independent package `routerforge-dns-router`, no dependency on `routerforge-dns`.
- Core `module_proxy.go` gains explicit `dns-router` Unix socket registration, installed package identity, `GET/POST/PUT` request policy, maximum mutation body bound, and same-origin + optional authenticated root gates. No Core listening port added.
- Core proxy route `/api/modules/dns-router/ui/index.html` -> `/v1/ui/index.html` and JS relative API path fixed. Management API retains CAS/atomic saves and import/export.
- Manual shadow-forwarder: real UDP/TCP DNS forwarding from explicit domain matches to explicit upstream IP:port, no system fallback for unmatched/local names, no transparent query leak. Any non-matching or local name responds REFUSED. Network/upstream failures respond SERVFAIL.
- Shadow listener is loopback-only (`127.0.0.1` or `::1`), port >=1025; **port 53 forbidden**, no NAT, no iptables/nftables, no Keenetic RCI mutations, no package postinstall hooks, no init script/autostart. Invocation requires `--serve --shadow-forward`.
- Management-only `--serve` preserves DPR2-B behavior. Shadow statistics in `GET /v1/shadow/status` and summary in `GET /v1/status`; no per-domain telemetry or query log.
- Existing P18 inside `routerforge-dns` stays untouched. DNS Router is a *separate feature, not traffic routing via PolicyN*.
- DNS Router remains **absent from release channel indices and public App Center** in DPR2-C. Core proxy wiring is only built/tested on the feature branch, not on installed Stable Core; the experimental IPK alone does not enable access through older Core.
- Experimental version: `0.14.0~dev.dpr2c`. Three target architecture IPKs are CI-smoke, not publishable public releases.

## Required gates

1. Local and CI Go unit tests + race (`go test -race`), Go vet.
2. Actual loopback UDP and TCP forwarding to fake test upstreams; refusal of local/unmatched names; forged response ID rejected; invalid/nonloopback/53 listener rejected.
3. Core module mapping, method allowlist, mutation gate, body size and UI path acceptance test.
4. Three architecture experimental IPKs with valid metadata and no autostart/postinst/port53 takeover. No new packages in Stable/Beta/Dev indices.
5. Feature branch only; exact base SHA, exact GitHub Actions Run ID, `gh run watch --exit-status`, no router connection or router mutations.

## Known limitations / DPR2-D

- The shadow listener does not handle system DNS/default upstream safely yet: unmatched names fail closed. No integration into Keenetic's standard resolver or :53; that must be separately designed and accepted.
- Single fixed, compiled snapshot of config at shadow startup. A saved config requires *manual restart* of the experimental process before being used by shadow forwarding.
- UDP truncated-response TCP retry, response QNAME compression, DoT/DoH, real fallback pool health, DNSSEC edge-cases, IPv6 interface acceptance and performance on hardware are pending.
- Installed package never starts service. Core proxy access needs manual management process and Core containing the DPR2-C feature patch; public App Center navigation/installation is a later milestone.
- Experimental IPK is compiled Go: in-use RAM and flash improvement is not claimed until measured hardware benchmarks.

## Manual sandbox invocation only (not for production DNS)

```
/opt/bin/routerforge-dns-router \
  --config /opt/etc/routerforge/dns-router/config.json \
  --serve --shadow-forward --shadow-listen 127.0.0.1:55353
```

No package installer or CI workflow executes this invocation on a router.
