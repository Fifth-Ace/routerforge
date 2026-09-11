# RouterForge release channels

| Channel | Source | Purpose |
| --- | --- | --- |
| Dev | `dev` | mutable ARM64 development train |
| Beta | exact `dev` FULL RELEASE | prerelease validation |
| Stable | exact validated SHA on `main` | production |

Current Stable: **0.7.1**.

Current package set:

```text
routerforge-core
routerforge-dns
routerforge-admin
routerforge-monitoring
routerforge-profiling
```

Targets: `aarch64-3.10` production; `mipsel-3.4` experimental with partial KN-1010 evidence; `mips-3.4` experimental without physical validation.

Source manifests: `dev.json`, `beta.json`, `stable.json`.
Release-index is authoritative for version, asset, URL, SHA256 and min-core metadata.

Stable promotion consumes the exact `routerforge-stable-promotion` artifact from a successful Dev FULL RELEASE for the same commit SHA.

See [`../../docs/RELEASE_PROCESS.md`](../../docs/RELEASE_PROCESS.md).
