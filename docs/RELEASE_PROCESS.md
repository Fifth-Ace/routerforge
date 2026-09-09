# RouterForge release process

This document is the public development/release contract for the current RouterForge train.

## Channels

| Channel | Source | Publication |
| --- | --- | --- |
| Dev | `dev` | Every successful push publishes mutable ARM64 `routerforge-dev`. |
| Beta | exact `dev` SHA | Explicit `workflow_dispatch` FULL RELEASE with `publish_beta=true`. |
| Stable | `main` | Promotion only after a successful FULL RELEASE artifact for the exact source SHA. |

`dev` is **not** the Beta release anymore. It is the continuously published development channel.

## Version ordering

Development and Beta package versions use the Debian/opkg prerelease separator `~`:

```text
0.7.1~dev.r248.4bcf08fdecc1
0.7.1~beta.1
0.7.1
```

Expected ordering:

```text
Dev < Beta < Stable
```

GitHub asset names replace `~` with `-`, while package control metadata keeps the real opkg version.

## Release-preparation gate

Before a Beta release:

1. `dev` must point at the exact intended source SHA.
2. tracked worktree and staging are clean before the release-preparation patch.
3. update `CHANGELOG.md`, public docs and `release/channels/beta.json`.
4. release tooling must accept the exact Beta component set.
5. consolidated Monitoring migration metadata/payload contract must pass CI.
6. `git diff --check` and targeted local checks must pass.
7. commit/push is a separate gate.
8. ordinary push CI must be green for the exact release-preparation SHA.
9. runtime-changing work must already have test-router evidence.

At the first RED gate: stop, diagnose, fix. Do not blind-rerun a release.

## Beta FULL RELEASE

The release is intentionally explicit:

```sh
gh workflow run .github/workflows/ci.yml \
  --ref dev \
  -f full_release=true \
  -f publish_beta=true
```

FULL RELEASE performs broad frontend/Go/shell/package/cross-build validation, builds Beta for all configured targets, verifies the consolidated Monitoring migration package contract, publishes the rolling `routerforge-beta` alias, verifies the published indexes/assets/bootstrap files, and then creates an immutable `routerforge-v<release_version>` snapshot.

For `0.7.1-beta.1` the expected package set per target is:

```text
routerforge-core
routerforge-dns
routerforge-admin
routerforge-monitoring
routerforge-profiling
```

Across ARM64 + MIPS + MIPSel this is 15 target-specific IPKs.

## Post-publication verification

Verify independently:

- workflow run SHA equals the release-preparation commit;
- all required jobs/critical steps are green;
- `routerforge-beta` indexes point at the expected versions/assets;
- `routerforge-beta-SHA256SUMS` covers exactly the current package assets;
- universal and target-specific bootstraps exist;
- immutable tag `routerforge-v<release_version>` points at the same exact SHA;
- immutable release assets are complete and are never clobbered;
- `main` / Stable remain unchanged during Beta publication.

## Monitoring migration hardware gate

A successful fresh install does not prove the `4 split packages → 1 consolidated package` upgrade path.

Before declaring the Beta migration verified, start from a router that still has one or more old split packages installed, update Monitoring through App Center, then run:

```sh
sh scripts/verify-monitoring-migration.sh runtime
```

For an exact Beta version check:

```sh
ROUTERFORGE_MONITORING_EXPECTED_VERSION='0.7.1~beta.1' \
  sh scripts/verify-monitoring-migration.sh runtime
```

PASS requires:

- `routerforge-monitoring` is installed;
- old RouterForge and historical dns-monitor monitoring packages are not installed;
- old split RouterForge binaries and init scripts are absent;
- no old split process is running;
- exactly the consolidated runtime owns the expected primary/compatibility service role;
- Monitoring plus System/Thermal/Storage/Network health endpoints answer through Core;
- all five Unix sockets exist. The four legacy-named sockets are intentional compatibility sockets, not leftovers.

If this gate is RED, do not manually `rm` package-owned files to make the check green. Diagnose the opkg migration state and fix the lifecycle contract.

## Hardware scope

ARM64 is the fully hardware-validated target.

MIPSel has partial physical validation on Keenetic Giga KN-1010: fresh installation and basic normal operation are confirmed. Upgrade/rollback/uninstall, full DNS/Module ABI behavior and resource-stress coverage remain open.

MIPS big-endian remains an experimental preview without physical hardware validation.

Cross-build, QEMU and runtime compatibility probes remain required for MIPS/MipSel, but they are not substitutes for the missing parts of the physical validation matrix.
