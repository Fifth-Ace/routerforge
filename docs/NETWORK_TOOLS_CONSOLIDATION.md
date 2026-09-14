# Network Tools consolidation audit

Date: 2026-09-14
Base Stable/Dev SHA before cleanup: `89c172f33e464d058f795aeb993f51ef14e56209`

## Decision
RouterForge 0.8.0 established `routerforge-network-tools` as the sole first-class network diagnostics module.

## KEEP
- `modules/network-tools/**`: dedicated runtime, UI and package.
- Core Network Tools descriptor and shell route.
- `modules/monitoring-runtime/network*.go`: read-only Monitoring telemetry.
- migration compatibility references to old split package names where they are required to detect/remove old installations.

## REMOVE
- Management `Network` tab.
- Management `adminNetworkToolRun` wrapper.
- Management `/v1/network-tools/run` backend route/implementation.
- dead `modules/network/packaging/S95routerforge-network` source stub.
- orphan `marketplace/approvals/network.json`.

## Guardrails
The Network Tools workflow verifies that the removed Management backend/UI symbols and dead legacy module source do not reappear. Related legacy/Admin paths now trigger the dedicated workflow.

## Deferred hardware gate
Hardware validation is deferred while the test router is unavailable. Before the next Beta/Stable promotion: install Dev on the Keenetic test router; verify Admin; verify all Network Tools tabs/probes; verify Monitoring network telemetry; confirm no old `routerforge-network` service is running; capture package/process/RSS/init-script state.