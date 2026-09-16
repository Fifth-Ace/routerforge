# Historical / superseded: RouterForge vNext module foundation

Status: **superseded**.

This file records an abandoned Concept R1 development direction. It is retained only as
historical context and must not be used as the current module plan, package topology, or
release contract.

The old concept proposed four new top-level packages: Maintenance, Network Tools,
Integrations, and Developer Tools. RouterForge did **not** keep that topology.

## Accepted architecture through Stable 0.9.0

```text
RouterForge
├── Core / App Center
├── Monitoring
├── DNS
├── Management
│   ├── Processes
│   ├── Services
│   ├── Packages / Ports
│   ├── File Manager
│   ├── Terminal
│   └── Maintenance
├── Network Tools
└── Profiling
```

Maintenance remains under Management rather than becoming a standalone top-level module.

Integrations remain nested in the Core/App Center integration surfaces rather than becoming
a standalone product module.

Developer Tools did not become a standalone product module.

Network Tools is the only new top-level module retained from the Concept R1 foundation.
It owns active diagnostics, has its own runtime/UI/package, and is separate from
Monitoring's read-only network telemetry.

Shared reusable source infrastructure now also includes `internal/safety`; reusable
source packages do not imply standalone daemons or product packages.

For the current product topology and ownership rules, use `docs/MODULES.md`,
`docs/REPOSITORY_LAYOUT.md`, and `docs/NETWORK_TOOLS_CONSOLIDATION.md`.
