# Repository layout

```text
components/core/                  Core backend/frontend/packaging
components/control/               Management backend/packaging
internal/safety/                  shared path/atomic/swap/command safety primitives
modules/admin/frontend/           Management UI
modules/dns/                      DNS runtime/frontend/packaging
modules/monitoring-runtime/       consolidated Monitoring runtime/packaging
modules/monitoring/frontend/      Monitoring UI
modules/network-tools/            standalone Network Tools runtime/UI/packaging
modules/nfqws-manager-runtime/    NFQWS Manager backend/runtime/packaging
modules/nfqws-manager/frontend/   NFQWS Manager UI
modules/profiling/packaging/      optional Core profiling lifecycle
release/channels/                 dev/beta/stable release topology
marketplace/                      public App Center registry sources/manifests
scripts/                          build/release/verification/failover tooling
docs/                             active and historical documentation
archive/                          explicitly archived historical material
```

Stable 0.10.0 release topology contains seven packages: Core, DNS, Admin, Monitoring,
Network Tools, NFQWS Manager and Profiling. Package versions remain independently governed;
the 0.10 train synchronizes published package versions to 0.10.0 because the public Beta
train already used 0.10.0~beta.x. See [VERSIONING.md](VERSIONING.md).

The legacy split source directories `modules/system`, `modules/thermal`, `modules/storage`,
and `modules/network` are intentionally absent. Compatibility package names, legacy init-script
names, and compatibility sockets may remain inside consolidated Monitoring metadata/verifiers
so upgrades from older installations can be detected and cleaned safely. They are not build targets.

`routerforge-network-tools` is the sole first-class network diagnostics package. Monitoring
may expose read-only network telemetry, but active probes and diagnostic tools belong to Network Tools.

`routerforge-nfqws-manager` owns RouterForge NFQWS management/diagnostics and the explicit
DPI Detector / NFQWS Menu integrations. The upstream nfqws2 package remains a separate project.

`marketplace/` remains root-level for public compatibility paths; the embedded Registry mirror
is CI-checked. Bundled manifests are the source of truth for integration metadata.

Channel manifests are `dev.json`, `beta.json`, and `stable.json`.
