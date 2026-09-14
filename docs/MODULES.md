# Official RouterForge modules

Stable 0.8.0 topology:

| Package | Purpose |
| --- | --- |
| `routerforge-core` | Web shell, auth, App Center, lifecycle and Module ABI host |
| `routerforge-dns` | DNS runtime, UI, control and diagnostics |
| `routerforge-admin` | Management: Processes, Services, Packages/Ports, File Manager, Terminal and Maintenance |
| `routerforge-monitoring` | Consolidated System/Thermal/Storage/Network telemetry |
| `routerforge-network-tools` | Network diagnostics: Doctor, probes, route inspection and flow exploration |
| `routerforge-profiling` | Loopback-only Core profiling |

## Ownership boundaries

`routerforge-admin` owns host administration and maintenance. It does not own network diagnostic probes.

`routerforge-network-tools` is the sole first-class network diagnostics module and owns active diagnostics plus its dedicated runtime/UI/package.

`routerforge-monitoring` remains read-only telemetry. Historical split package names such as `routerforge-network` may remain in migration compatibility metadata so old installations can be detected and removed; they are not active product modules.

## Versions

Stable 0.8.0 is a six-package product topology with independent component versioning. Release-index metadata is authoritative for exact assets and SHA256 values.