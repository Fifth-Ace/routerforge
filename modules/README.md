# RouterForge modules

Current Stable 0.9.0 topology:

- `dns/` - independent DNS Module ABI v1 runtime/frontend/packaging, Stable package 0.8.1.
- `admin/frontend/` - Management UI; backend/packaging live under `components/control/`, Stable package 0.8.1.
- `monitoring-runtime/` - consolidated System/Thermal/Storage/Network telemetry runtime and package lifecycle, Stable package 0.8.0.
- `monitoring/frontend/` - Monitoring UI.
- `network-tools/` - standalone Network Tools runtime/UI/packaging, Stable package 0.9.0.
- `profiling/` - optional Core profiling lifecycle, unchanged Stable package 0.7.1.

Legacy split source directories `system/`, `thermal/`, `storage/`, and `network/` are
intentionally absent. Their historical package names and compatibility sockets remain only
where the consolidated Monitoring migration contract needs them; they are not active build
targets or product modules.

Components are independently versioned; see [`../docs/VERSIONING.md`](../docs/VERSIONING.md).

See [`../docs/REPOSITORY_LAYOUT.md`](../docs/REPOSITORY_LAYOUT.md).
