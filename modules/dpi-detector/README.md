# RouterForge DPI Detector

RouterForge integration for the upstream **DPI Detector** project by **Runnin4ik**.

Upstream repository: https://github.com/Runnin4ik/dpi-detector

## Product model

`routerforge-dpi-detector` is a separate package with an independent upstream
version and update lifecycle. Its user interface is integrated into
**NFQWS → DPI Detector**.

Two access modes are intentional:

1. **Console** — upstream-style CLI via `/opt/bin/dpi-detector`.
2. **Web** — RouterForge visual orchestration through the nfqws-manager API.

RouterForge does not claim ownership of the upstream project. The upstream MIT
license is shipped in the package, along with RouterForge's own license.

## Update lifecycle

The authoritative pin lives in `modules/dpi-detector/upstream.json`.

For a new upstream release:

1. Detect the new upstream release.
2. Review its tag and exact commit SHA.
3. Update the pin in `upstream.json`.
4. CI rebuilds an Entware-compatible ARM64 package in a pinned build environment.
5. Package, CLI and RouterForge integration gates run.
6. The candidate is published to the dedicated development release only after PASS.
7. Hardware acceptance is performed before promotion to the normal module channel.

The upstream release is therefore not silently pushed to routers.