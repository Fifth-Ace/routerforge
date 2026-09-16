# Official RouterForge modules

Stable 0.9.0 topology:

| Package | Version | Purpose |
| --- | ---: | --- |
| `routerforge-core` | **0.9.0** | Web shell, auth, App Center, Registry/release lifecycle, Module ABI host |
| `routerforge-dns` | **0.8.1** | DNS runtime, UI, resolver control, rolling health and diagnostics |
| `routerforge-admin` | **0.8.1** | Management: Processes, Services, Packages/Ports, File Manager, Terminal and Maintenance |
| `routerforge-monitoring` | **0.8.0** | Consolidated System/Thermal/Storage/Network telemetry |
| `routerforge-network-tools` | **0.9.0** | Network Doctor, Route Inspector, Flow Explorer and active diagnostics |
| `routerforge-profiling` | **0.7.1** | Loopback-only Core profiling |

## Ownership boundaries

`routerforge-admin` owns host administration and maintenance. It does not own network diagnostic probes.

`routerforge-network-tools` is the sole first-class network diagnostics module and owns
active diagnostics plus its dedicated runtime/UI/package.

`routerforge-monitoring` remains read-only telemetry. Historical split package names such as
`routerforge-network` may remain in migration compatibility metadata so old installations
can be detected and removed; they are not active product modules.

## Compatibility

DNS/Admin/Monitoring/Network Tools require Core `0.9.0`.
Profiling is unchanged and keeps `min_core_version=0.7.1`.

## Versions

RouterForge components are independently versioned. Stable 0.9.0 does not force every
package to `0.9.0`; only changed components are bumped according to
[VERSIONING.md](VERSIONING.md). Release-index metadata is authoritative for exact assets
and SHA256 values.
