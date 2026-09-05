# RouterForge modules

- `dns/` — independent DNS Module ABI v1 runtime, frontend and packaging.
- `system/`, `thermal/`, `storage/`, `network/` — package ownership for the read-only monitoring modules.
- `monitoring-runtime/` — shared Go runtime currently used by System/Thermal/Storage/Network.
- `profiling/` — optional Core profiling package/configuration.

See [`../docs/REPOSITORY_LAYOUT.md`](../docs/REPOSITORY_LAYOUT.md) for ownership rules and the shared-runtime rationale.
