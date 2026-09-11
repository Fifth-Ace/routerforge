# Официальные модули RouterForge

Stable 0.7.1:

| Package | Назначение |
| --- | --- |
| `routerforge-core` | Web shell/auth/App Center/lifecycle/Module ABI host |
| `routerforge-dns` | DNS runtime/UI/control/diagnostics |
| `routerforge-admin` | Management v2/File Manager/Maintenance/terminals |
| `routerforge-monitoring` | consolidated System/Thermal/Storage/Network |
| `routerforge-profiling` | loopback-only Core profiling |

## DNS
Independent Module ABI v1 runtime. Resolver writes сохраняют snapshot/save/readback/rollback semantics.

## Management
`routerforge-admin` предоставляет Processes, Services, File Manager, Maintenance, Entware Terminal и Keenetic NDM Console. Process/service/file mutations защищены root-session/same-origin/confirmation/internal-marker gates. Keenetic mode использует fixed server-side `ndmc`.

Подробнее: [MANAGEMENT_V2_API.md](MANAGEMENT_V2_API.md) и [MANAGEMENT_V2_FILES_API.md](MANAGEMENT_V2_FILES_API.md).

## Monitoring
`routerforge-monitoring` заменяет split packages. Legacy package names остаются только migration compatibility через `Provides/Conflicts/Replaces`.

## Profiling
Default: `127.0.0.1:6061`.

## Versions
Stable 0.7.1 — coherent five-package train. Release-index authoritative для exact assets/SHA256; architecture проекта по-прежнему допускает independent component versioning в будущих component-only releases.
