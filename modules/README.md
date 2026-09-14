# RouterForge modules

Current Stable 0.8.0 topology:

- `dns/` - independent DNS Module ABI v1 runtime/frontend/packaging.
- `admin/frontend/` - Management UI; backend/packaging live under `components/control/`.
- `monitoring-runtime/` - consolidated System/Thermal/Storage/Network telemetry runtime and package lifecycle.
- `monitoring/frontend/` - Monitoring UI.
- `network-tools/` - standalone Network Tools runtime/UI/packaging.
- `profiling/` - optional Core profiling lifecycle.

Legacy split source directories `system/`, `thermal/`, `storage/`, and `network/` are intentionally absent. Their historical package names and compatibility sockets remain only where the consolidated Monitoring migration contract needs them; they are not active build targets or product modules.

See [`../docs/REPOSITORY_LAYOUT.md`](../docs/REPOSITORY_LAYOUT.md).
