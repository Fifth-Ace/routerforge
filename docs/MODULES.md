# Official RouterForge modules

Stable 0.10.0 topology:

| Package | Version | Purpose |
| --- | ---: | --- |
| `routerforge-core` | **0.10.0** | Web shell, auth, App Center, Registry/release lifecycle, Module ABI host |
| `routerforge-dns` | **0.10.0** | DNS runtime, UI, resolver control, rolling health and diagnostics |
| `routerforge-admin` | **0.10.0** | Management: Processes, Services, File Manager, Terminal, Backup/Restore and Maintenance |
| `routerforge-monitoring` | **0.10.0** | Consolidated System/Thermal/Storage/Network telemetry and process manager |
| `routerforge-network-tools` | **0.10.0** | Network Doctor, Route Inspector, Flow Explorer and active diagnostics |
| `routerforge-nfqws-manager` | **0.10.0** | nfqws2 management, diagnostics, strategy selection, DPI Detector/NFQWS Menu integrations |
| `routerforge-profiling` | **0.10.0** | Loopback-only Core profiling |

## Ownership boundaries

`routerforge-admin` owns host administration, services, files, terminal and recovery workflows. It does not own network diagnostic probes.

`routerforge-network-tools` is the sole first-class network diagnostics module and owns active diagnostics plus its dedicated runtime/UI/package.

`routerforge-nfqws-manager` owns RouterForge integration with an existing `nfqws2-keenetic` runtime. It does not silently install or replace nfqws2.

`routerforge-monitoring` remains read-only telemetry. Historical split package names such as `routerforge-network` may remain in migration compatibility metadata so old installations can be detected and removed; they are not active product modules.

## Compatibility

DNS/Admin/Monitoring/Network Tools/NFQWS Manager require Core `0.10.0`.
Profiling package version is synchronized to `0.10.0` for the 0.10 release train but keeps `min_core_version=0.7.1`.

## Versions

RouterForge components remain independently versioned by policy. The 0.10 release train intentionally synchronizes the seven published package versions to `0.10.0` because the public prerelease train used `0.10.0~beta.x`; future trains should avoid cosmetic prerelease bumps for unchanged components.

Release-index metadata is authoritative for exact assets and SHA256 values.
