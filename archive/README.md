# Archive

This directory contains historical RouterForge / DNS Monitor material that is intentionally kept for reference but is **not part of the active build, package or runtime layout**.

Archived files must not be referenced by current CI, build scripts, package manifests or installation documentation.

## Contents

- `legacy-direct-install/` — pre-channel direct installation helpers kept only for history.
- `branding/` — superseded branding assets.
- `legacy-source/` — source files proven unused by active Core/module build paths and retained only for history.

Important: not every `dns-monitor-*` string in the active repository is obsolete. Some names are still required for package migration (`Provides / Conflicts / Replaces`) and historical repository compatibility. Those compatibility references remain active and must not be moved here.
