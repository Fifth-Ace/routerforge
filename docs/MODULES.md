# Official RouterForge modules

Stable 0.11.0 topology:

| Package | Version | Purpose |
| --- | ---: | --- |
| `routerforge-core` | **0.10.1** | Web shell, auth, App Center, Registry/release lifecycle, Module ABI host; Geo Split status fix |
| `routerforge-dns` | **0.10.0** | DNS runtime, UI, resolver control, rolling health and diagnostics |
| `routerforge-admin` | **0.10.0** | Management: Processes, Services, File Manager, Terminal, Backup/Restore and Maintenance |
| `routerforge-monitoring` | **0.10.0** | Consolidated System/Thermal/Storage/Network telemetry and process manager |
| `routerforge-network-tools` | **0.10.0** | Network Doctor, Route Inspector, Flow Explorer and active diagnostics |
| `routerforge-nfqws-manager` | **0.10.0** | nfqws2 management, diagnostics, strategy selection, DPI Detector/NFQWS Menu integrations |
| `routerforge-antiscan-manager` | **0.11.0** | Guarded management for an existing upstream Antiscan installation |
| `routerforge-profiling` | **0.10.0** | Loopback-only Core profiling |

## Ownership boundaries

`routerforge-admin` owns host administration, services, files, terminal and recovery workflows. It does not own network diagnostic probes.

`routerforge-network-tools` is the sole first-class network diagnostics module and owns active diagnostics plus its dedicated runtime/UI/package.

`routerforge-nfqws-manager` owns RouterForge integration with an existing `nfqws2-keenetic` runtime. It does not silently install or replace nfqws2.

`routerforge-antiscan-manager` owns RouterForge management/diagnostics for an existing `dimon27254/antiscan` installation. It does not install, update or replace upstream Antiscan.

`routerforge-monitoring` remains read-only telemetry. Historical split package names such as `routerforge-network` may remain in migration compatibility metadata so old installations can be detected and removed; they are not active product modules.

## Compatibility

DNS/Admin/Monitoring/Network Tools/NFQWS Manager require Core `0.10.0`.
Antiscan Manager `0.11.0` requires Core `0.10.1`.
Profiling remains `0.10.0` with `min_core_version=0.7.1`.

## Versions

RouterForge components remain independently versioned by policy. Stable 0.11.0 demonstrates the rule directly: Core is `0.10.1`, Antiscan Manager is `0.11.0`, and unchanged packages remain `0.10.0`.

Release-index metadata is authoritative for exact assets and SHA256 values.
