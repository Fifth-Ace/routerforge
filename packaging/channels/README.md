# RouterForge release channels

Each RouterForge component is versioned independently.

- `beta.json` is published from `dev` to the `routerforge-beta` release.
- `stable.json` is published from `main` to the `routerforge-stable` release.

To release only one module, bump only that component's `version`.
Do not bump Core unless Core itself changed and should be deployed.

The CI release index is authoritative for:
- available version
- exact asset filename / URL
- SHA256
- minimum Core version metadata

If a component version did not change, CI keeps the previously published
asset and checksum instead of silently replacing it.
