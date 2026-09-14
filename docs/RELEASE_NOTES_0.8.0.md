# RouterForge 0.8.0

Stable 0.8.0 promotes the accepted 0.8.0 Beta workstream into the production channel.

## Package versions

| Package | Stable version | Status |
| --- | --- | --- |
| `routerforge-core` | `0.8.0` | changed |
| `routerforge-dns` | `0.8.0` | changed since Stable 0.7.2 |
| `routerforge-admin` | `0.8.0` | changed |
| `routerforge-monitoring` | `0.7.1` | unchanged |
| `routerforge-network-tools` | `0.8.0` | new Stable module |
| `routerforge-profiling` | `0.7.1` | unchanged |

## Highlights

- standalone Network Tools inside the RouterForge shell;
- configurable CPU temperature warning threshold, default 75 °C;
- corrected module iframe sizing for Network Tools and Management;
- Files and Terminal again fill the available desktop workspace;
- six-component Stable topology across ARM64, MIPS and MIPSel release indexes.

## Promotion contract

Stable 0.8.0 is promoted only from an exact validated Dev SHA using a successful FULL RELEASE
with `publish_beta=false`. The exact `routerforge-stable-promotion` artifact is then consumed
by the main-branch publication job. An immutable `routerforge-v0.8.0` snapshot is created last.
