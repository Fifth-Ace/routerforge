# Repository layout

```text
components/core/                  Core backend/frontend/packaging
components/control/               Management backend/packaging
modules/admin/frontend/           Management UI
modules/dns/                      DNS runtime/frontend/packaging
modules/monitoring-runtime/       consolidated Monitoring runtime/packaging
modules/monitoring/frontend/      Monitoring UI
modules/network-tools/            standalone Network Tools runtime/UI/packaging
modules/profiling/packaging/      optional Core profiling lifecycle
release/channels/                 dev/beta/stable release topology
marketplace/                      public App Center registry sources
scripts/                          build/release/verification tooling
docs/                             active and historical documentation
archive/                          explicitly archived historical material
```

Stable 0.8.0 release topology is six packages: Core, DNS, Admin, Monitoring, Network Tools, Profiling.

The legacy split source directories `modules/system`, `modules/thermal`, `modules/storage`, and `modules/network` are intentionally absent. Compatibility package names, legacy init-script names, and compatibility sockets may remain inside the consolidated Monitoring package metadata and migration verifiers so upgrades from older installations can be detected and cleaned safely. They are not build targets.

`routerforge-network-tools` is the sole first-class network diagnostics package. Monitoring may expose read-only network telemetry, but active probes and diagnostic tools belong to Network Tools.

`marketplace/` remains root-level for public compatibility paths; the embedded Registry mirror is CI-checked.

Channel manifests are `dev.json`, `beta.json`, and `stable.json`.
