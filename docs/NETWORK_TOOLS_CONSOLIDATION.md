# Network Tools consolidation audit

Original consolidation decision: 2026-09-14
Stable baseline before cleanup: `89c172f33e464d058f795aeb993f51ef14e56209`

## Decision

RouterForge 0.8.0 established `routerforge-network-tools` as the sole first-class
network diagnostics module. Stable 0.9.0 keeps that ownership and advances the package
to **0.9.0** after the substantial Route Inspector/runtime/UI expansion.

## KEEP
- `modules/network-tools/**`: dedicated runtime, UI and package.
- Core Network Tools descriptor and shell route.
- `modules/monitoring-runtime/network*.go`: read-only Monitoring telemetry.
- migration compatibility references to old split package names where required to detect/remove old installations.

## REMOVE / remains removed
- Management `Network` tab.
- Management `adminNetworkToolRun` wrapper.
- Management `/v1/network-tools/run` backend route/implementation.
- dead `modules/network/packaging/S95routerforge-network` source stub.
- orphan `marketplace/approvals/network.json`.

## Stable 0.9.0 additions
- Expanded Network Tools runtime/UI.
- Route Inspector has its own implementation and tests.
- Network Doctor/routes/flows/active probes remain under the standalone Network Tools boundary.

## Guardrails

The Network Tools workflow verifies that removed Management backend/UI symbols and dead
legacy module source do not reappear. Related legacy/Admin paths trigger the dedicated workflow.

## Hardware status

The previously deferred hardware gate has been exercised through the current Dev/Beta
hardware cycle before Stable 0.9.0 release preparation. Network Tools remains part of the
ARM64 Beta/Stable acceptance path. MIPS/MIPSel status remains as documented in
[ARCHITECTURES.md](ARCHITECTURES.md).
