# Monitoring migration: split packages → `routerforge-monitoring`

RouterForge 0.7.1 Beta replaces the four split monitoring packages with one consolidated package.

## Old topology

```text
routerforge-system
routerforge-thermal
routerforge-storage
routerforge-network
```

Historical installations can also contain the corresponding `dns-monitor-*` predecessors.

## New topology

```text
routerforge-monitoring
```

One process serves the consolidated Monitoring UI/runtime and compatible System/Thermal/Storage/Network APIs.

Expected sockets after a successful migration:

```text
/opt/var/run/routerforge-monitoring.sock
/opt/var/run/routerforge-system.sock
/opt/var/run/routerforge-thermal.sock
/opt/var/run/routerforge-storage.sock
/opt/var/run/routerforge-network.sock
```

The four legacy-named sockets are **intentional compatibility sockets**. They are not package leftovers.

## Package migration contract

`routerforge-monitoring` declares the legacy monitoring package names in `Provides`, `Conflicts` and `Replaces`.

The new package payload owns only the consolidated binary/init/UI/manifest. It does not ship the old split binaries or init scripts.

During `postinst` the package also stops any still-running split RouterForge services and removes stale socket files before starting the consolidated runtime.

## Hardware verification

After upgrading through App Center, run the repository verifier from a checked-out copy or copy the script to the router:

```sh
ROUTERFORGE_MONITORING_EXPECTED_VERSION='0.7.1~beta.1' \
  sh scripts/verify-monitoring-migration.sh runtime
```

The verifier is read-only. It does not remove packages or files.

A PASS proves:

- new package is installed at the expected version when one is supplied;
- old RouterForge and historical dns-monitor monitoring packages are not installed;
- old split RouterForge binaries/init scripts are absent;
- old split RouterForge processes are not running;
- consolidated binary/service/UI/manifest are present;
- primary + four compatibility sockets exist;
- Monitoring and compatibility health endpoints answer through Core.

At the first FAIL, stop. Do not manually delete files: package DB state and filesystem state must agree naturally after the supported migration.
