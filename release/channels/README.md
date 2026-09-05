# RouterForge release channels

Each RouterForge component is versioned independently.

- `beta.json` is published from `dev` to the `routerforge-beta` Pre-release.
- `stable.json` is published from `main` to the `routerforge-stable` production release.

To release only one module, bump only that component's `version`.
Do not bump Core unless Core itself changed and should be deployed.

The CI release index is authoritative for:

- available version;
- exact asset filename / URL;
- SHA256;
- minimum Core version metadata.

If a component version did not change, CI keeps the previously published asset and checksum instead of silently replacing a same-version binary.

Each published channel also contains:

```text
routerforge-<channel>-index.json
routerforge-<channel>-SHA256SUMS
routerforge-<channel>-bootstrap.sh
```

The bootstrap script is generated from the final merged release index, so Core and DNS can be installed even when their versions differ.
