# RouterForge Marketplace Registry

`marketplace/registry/index.json` is the single catalog document consumed by every local RouterForge instance. The Core ships with an embedded copy, caches the last remote copy and refreshes the public registry asynchronously.

## Developer submission flow

1. Fork the registry repository.
2. Add `marketplace/submissions/<id>.json` using `schema/manifest.schema.json`.
3. Run `python3 marketplace/build_registry.py`.
4. Open a pull request containing the submission and regenerated `registry/index.json`.
5. CI validates ids, package names and lifecycle step types. A new manifest without approval is published as `UNVERIFIED` after merge.

The registry never accepts raw shell command strings. Lifecycle is declared as structured actions (`opkg-*`, `write-opkg-feed`) or as an explicitly typed upstream installer that requires a later executor/review policy.

## Approval model

Human review lives separately in `marketplace/approvals/<id>.json` and pins the canonical SHA256 of the reviewed manifest.

- `OFFICIAL` — RouterForge-owned module.
- `VERIFIED` — third-party lifecycle reviewed against upstream documentation.
- `UNVERIFIED` — schema-valid submission without human approval.
- `CHANGED` — manifest hash changed after approval.
- `BLOCKED` — execution disabled by registry maintainers.
- `DEPRECATED` — retained for discovery, automatic actions disabled.

Changing a manifest automatically invalidates the old approval because its canonical SHA256 no longer matches.

## Current dev transport

The dev Core reads:

`https://raw.githubusercontent.com/Fifth-Ace/routerforge/dev/marketplace/registry/index.json`

RouterForge-owned optional IPKs are published to the rolling GitHub prerelease tag `routerforge-dev`. Only the selected IPK is downloaded to `/opt/tmp/routerforge-marketplace`, verified against `routerforge-dev-SHA256SUMS`, installed, and deleted immediately.

A dedicated registry repository/domain and Ed25519-signed indexes are planned before production Marketplace execution is enabled outside the dev gate.
