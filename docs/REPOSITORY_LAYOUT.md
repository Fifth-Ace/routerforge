# Repository layout

```text
components/core/             Core backend/frontend/packaging
components/control/          Management backend/packaging
modules/admin/frontend/      Management UI
modules/dns/                 DNS runtime/frontend/packaging
modules/monitoring-runtime/  consolidated Monitoring runtime
modules/monitoring/frontend/ Monitoring UI
modules/system/packaging/    legacy migration ownership
modules/thermal/packaging/   legacy migration ownership
modules/storage/packaging/   legacy migration ownership
modules/network/packaging/   legacy migration ownership
modules/profiling/packaging/
release/channels/
marketplace/
scripts/
docs/
archive/
```

Stable 0.7.1 release topology: Core, DNS, Admin, Monitoring, Profiling.

Split monitoring directories remain only for migration/compatibility lifecycle. Product source should live with its owning component/module, not flat at repository root.

`marketplace/` remains root-level for public compatibility paths; embedded Registry mirror is CI-checked.

Channel manifests: `dev.json`, `beta.json`, `stable.json`.
