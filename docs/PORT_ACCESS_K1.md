# Port Access Manager — K1 foundation

K1 is **read-only**. It does not install `knockd` or `fwknopd`, start services,
read cryptographic keys, modify iptables/ipset or open ports.

The runtime exposes a local Unix-socket HTTP API:

- `GET /v1/health` — module/version/read-only status
- `GET /v1/status` — availability of both engines, package presence, daemon process, config-file presence and service presence
- `GET /v1/ui/index.html` — static Russian UI

All mutating methods return 405. The runtime is intentionally not registered
in the published package feed or Core module catalog by this patch. That integration
must be implemented in K2 after Core-side proxy and module security gates have
been reviewed. This avoids presenting a dead module as installable.

Follow-up gates:
1. K2: official module registry + package build + protected Core proxy + install preview.
2. K3: `knockd` configuration and atomic preview/apply with safe timeout cleanup.
3. K4: firewall hook under Keenetic ACL, preserving existing Antiscan priority and SSH rescue path.
4. K5: `fwknopd` securely provisioned adapter and explicit install.
5. K6: combined grants table, audits and hardware acceptance.

Do not allow either daemon to manage arbitrary firewall rules until the
firewall-hook and recovery contract has been proven on test Keenetic hardware.
