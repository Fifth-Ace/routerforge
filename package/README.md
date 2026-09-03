# RouterForge beta

RouterForge is a modular local router console for Keenetic / Netcraze ARM64 + Entware.

Core UI listens on port **2233**.

Beta package namespace:

- `routerforge-core`
- `routerforge-dns`
- `routerforge-admin`
- `routerforge-system`
- `routerforge-thermal`
- `routerforge-storage`
- `routerforge-network`
- `routerforge-profiling`

Legacy `dns-monitor-*` package names are migrated through opkg `Provides / Conflicts / Replaces`.
Optional packages are fetched on demand from the rolling `routerforge-beta` GitHub release.
