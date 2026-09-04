# RouterForge architectures

RouterForge packages are Entware IPKs. The package architecture and the Go target are related, but they are not the same string.

## Release status

| RouterForge target | Go target | Entware feed family | Release status |
| --- | --- | --- | --- |
| `aarch64-3.10` | `GOARCH=arm64` | `aarch64-k3.10` | **Stable / Beta supported** |
| `mips-3.4` | `GOARCH=mips`, `GOMIPS=softfloat` | `mipssf-k3.4` | **Planned; CI cross-build only** |
| `mipsel-3.4` | `GOARCH=mipsle`, `GOMIPS=softfloat` | `mipselsf-k3.4` | **Planned; CI cross-build only** |

Current bootstrap and rolling release indexes still publish `aarch64-3.10` assets only.

The MIPS profiles exist now so build scripts do not have to be redesigned later. They are deliberately **not advertised as supported installs** until each target is validated on real Keenetic / Netcraze hardware.

## Build target selection

Default:

```sh
./scripts/build-opkg.sh 0.0.0-dev
```

Explicit aarch64:

```sh
ROUTERFORGE_TARGET=aarch64-3.10 ./scripts/build-opkg.sh 0.0.0-dev
```

MIPS compile/package smoke:

```sh
ROUTERFORGE_TARGET=mips-3.4 ./scripts/build-opkg.sh 0.0.0-dev
ROUTERFORGE_TARGET=mips-3.4 ./scripts/build-module-opkg.sh dns 0.0.0-dev
```

MIPSEL compile/package smoke:

```sh
ROUTERFORGE_TARGET=mipsel-3.4 ./scripts/build-opkg.sh 0.0.0-dev
ROUTERFORGE_TARGET=mipsel-3.4 ./scripts/build-module-opkg.sh dns 0.0.0-dev
```

Target mapping lives in `scripts/target-env.sh`.

## Before enabling MIPS releases

Do not add MIPS assets to `routerforge-stable` or `routerforge-beta` until all of the following are done:

1. Core, Control, DNS and generic monitoring runtimes cross-build in CI.
2. IPK `Architecture:` matches the Entware target.
3. Core starts on real hardware and `/api/health` is healthy.
4. Module ABI Unix sockets work on real hardware.
5. DNS discovery/capture/control is verified against that KeeneticOS generation.
6. Marketplace selects an asset matching the local opkg architecture.
7. Bootstrap detects the local architecture and refuses unsupported targets.
8. Install, update, rollback and uninstall are tested independently for MIPS and MIPSEL.

The future release-index change should be additive and backward-compatible with existing aarch64 Core versions.
