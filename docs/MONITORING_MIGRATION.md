# Monitoring migration: split packages → `routerforge-monitoring`

RouterForge **0.7.1 Stable** replaces `routerforge-system`, `routerforge-thermal`, `routerforge-storage`, `routerforge-network` with `routerforge-monitoring`.

Expected sockets:

```text
routerforge-monitoring.sock
routerforge-system.sock
routerforge-thermal.sock
routerforge-storage.sock
routerforge-network.sock
```

The four legacy-named sockets are intentional compatibility sockets.

`routerforge-monitoring` uses `Provides/Conflicts/Replaces`, stops split services and removes stale sockets during migration.

ARM64 4→1 migration has hardware evidence including package cleanup, runtime health and reboot/autostart.

Verifier:

```sh
ROUTERFORGE_MONITORING_EXPECTED_VERSION='0.7.1' \
  sh scripts/verify-monitoring-migration.sh runtime
```

The verifier accepts valid Entware `Status: install <selection> installed` forms.

At first FAIL stop and diagnose package DB/filesystem; do not manually delete package-owned files just to force green.
