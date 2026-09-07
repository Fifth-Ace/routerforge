# RouterForge release channels

Each RouterForge component is versioned independently.

- `beta.json` is published from `dev` to the `routerforge-beta` Pre-release.
- `stable.json` is published from `main` to the `routerforge-stable` production release.

To release only one module, bump only that component's `version`.
Do not bump Core unless Core itself changed and should be deployed.

The CI release index is authoritative for:

- available version;
- exact asset filename / URL;
- SHA256;
- minimum Core version metadata.

If a component version did not change, CI keeps the previously published asset and checksum instead of silently replacing a same-version binary.

Each published channel also contains:

```text
routerforge-<channel>-index.json
routerforge-<channel>-SHA256SUMS
routerforge-<channel>-bootstrap.sh
```

The bootstrap script is generated from the final merged release index and installs RouterForge Core only. Optional RouterForge modules are selected afterwards from App Center; their versions remain independent from Core and from each other.

Stable 0.6 and Beta publish target-specific indexes for:

```text
aarch64-3.10
mips-3.4
mipsel-3.4
```

AArch64 is the hardware-validated target. MIPS/MipSel are published as experimental previews
only: they pass cross-build/QEMU/runtime-probe gates but have not been validated on physical
hardware. Their target bootstrap keeps the explicit preview opt-in and fail-closed runtime probe.
