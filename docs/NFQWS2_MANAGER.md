# RouterForge P24 — NFQWS2 Manager

Installed-only adapter for an existing `nfqws2-keenetic` installation.

## Read surface
- detection/running state;
- exact init/config/list/log paths;
- current config (bounded 128 KiB);
- SHA-256 of current config;
- bounded list inventory/preview (32 KiB each);
- bounded log tail (64 KiB);
- safety backup count.

## Mutation surface
All mutations are reachable only through the existing Core-guarded Admin mutation boundary.

Supported:
- reload;
- restart;
- config save.

Config save:
1. exact installed-runtime precheck;
2. bounded content validation;
3. safety backup in `/tmp/routerforge-nfqws2-backups`;
4. atomic write of `/opt/etc/nfqws2/nfqws2.conf`;
5. `S51nfqws2 reload`;
6. if reload fails, restore previous bytes atomically and attempt reload again;
7. keep at most 8 backups.

No package install/update and no automatic DPI strategy generation are part of P24.
