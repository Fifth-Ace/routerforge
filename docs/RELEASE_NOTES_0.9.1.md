# RouterForge 0.9.1

Stable 0.9.1 is a focused installer/bootstrap hotfix on top of Stable 0.9.0.

## Fixed

- Missing optional thermal sensors on MIPS/MIPSel no longer block Core installation.
- Thermal capability is reported as unsupported when no sensor is available.
- Core/platform installation gating is separated from optional module status.
- Real platform prerequisites remain fail-closed.
- Bootstrap verifies local RouterForge Core /api/health after installation.

## Scope boundary

This release intentionally did **not** include P16 Config Vault, P17 completion, or P18 DNS Policy Router from the then-current Dev/Beta feature line.

Component package versions stayed unchanged from Stable 0.9.0:

- Core 0.9.0
- DNS 0.8.1
- Admin 0.8.1
- Monitoring 0.8.0
- Network Tools 0.9.0
- Profiling 0.7.1

## Multiarch

- aarch64-3.10: production target.
- mips-3.4: experimental.
- mipsel-3.4: experimental; KN-2311 Hero 4G+ confirmed Core 0.9.0 install, UI and runtime on physical hardware.
