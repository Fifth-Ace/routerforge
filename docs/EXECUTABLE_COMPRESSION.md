# Executable Compression

RouterForge AArch64 production packages use UPX compression for executable
payloads.

## Policy

For target:

`aarch64-3.10`

Go executables are packed with:

`upx --ultra-brute`

Every packed executable is immediately verified with:

`upx -t`

A packing or integrity failure fails the package build.

Other architectures remain uncompressed until they have completed equivalent
hardware runtime validation.

## Why

Whole-stack AArch64 hardware attribution measured the complete seven-process
RouterForge production executable set.

Plain executables:

- 37.562 MiB

UPX-ultra executables:

- 12.410 MiB

Installed executable storage reduction:

- 25.153 MiB
- 66.962%

Seven-run aggregate resident-memory comparison:

- plain: 38,161.7 KiB VmRSS
- UPX ultra: 44,296.0 KiB VmRSS
- delta: +6,134.3 KiB
- delta: +16.074%

All tested packed executables passed UPX integrity verification and started
successfully on the AArch64 Keenetic test router. Core HTTP health remained
healthy.

The measured storage/RAM trade was accepted for the targeted router class.

## Diagnostic plain builds

An intentional plain AArch64 diagnostic build can be requested with:

`ROUTERFORGE_UPX=0`

This is an escape hatch for diagnosis and does not change the production
default.

A custom UPX executable can be supplied with:

`ROUTERFORGE_UPX_BIN=/path/to/upx`

CI pins the validated production tool version rather than relying on an
uncontrolled distribution package.

## MIPS / MIPSel

The AArch64 result must not be generalized to MIPS or MIPSel.

Those targets remain plain until separately validated on corresponding
hardware.