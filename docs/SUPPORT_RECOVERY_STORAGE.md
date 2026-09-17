# Support, Recovery & Storage

RouterForge P22 consolidates recovery state, storage health and support diagnostics into the existing Admin/Maintenance workspace.

## Support bundle

The support bundle is a bounded ZIP created under `/tmp/routerforge-support`.

It contains:
- RouterForge/Admin version and device summary;
- storage and thermal telemetry;
- service state without process command-line arguments;
- integration state;
- watchdog state with redacted output;
- a redacted bounded log tail;
- a manifest describing privacy exclusions.

It intentionally does **not** include:
- `/opt/etc/routerforge` configuration files;
- process command-line arguments;
- cron command bodies;
- private-key blocks;
- obvious password/token/API-key/authorization values from log text.

Limits:
- maximum source data: 4 MiB;
- maximum single generated entry: 2 MiB;
- log tail: 128 KiB;
- retained bundles: 8;
- support files are created mode `0600`.

## Storage health

Storage is summarized with simple operational thresholds:
- below 80% used: `ok`;
- 80–89.9%: `warning`;
- 90% or more: `critical`.

This is diagnostic status only. P22 does not automatically delete data or remount filesystems.

## Recovery

P22 reuses the existing guarded RouterForge recovery mechanisms:
- config-only backup and restore;
- automatic pre-restore safety backup;
- Config Vault pre-change snapshot;
- diagnostic snapshots;
- opt-in watchdogs with cooldown and hourly attempt limits.

No automatic restart is added to config restore.
