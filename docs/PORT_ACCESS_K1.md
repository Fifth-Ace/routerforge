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

## Community adapter: iptables recent (odin)

Original approach by **odin**, Keenetic Community, 3 August 2025:
https://forum.keenetic.ru/topic/22137-port-knoking-%D0%B4%D0%BB%D1%8F-%D0%BF%D1%80%D0%BE%D0%B1%D1%80%D0%B0%D1%81%D1%8B%D0%B2%D0%B0%D0%B5%D0%BC%D0%BE%D0%B3%D0%BE-%D0%BF%D0%BE%D1%80%D1%82%D0%B0/

The proposal protects an existing Keenetic port-forward with a 3-port knock
sequence, wrong-port reset, and time-limited access using `xt_recent`.
RouterForge credits the author and plans an independently implemented adapter.
We have not copied the forum shell script or assumed it grants a reuse license.

Current integration: **read-only** kernel match detection via
`/proc/net/ip_tables_matches`. A positive value means `recent` match is loaded;
it does NOT mean the forward is protected, that a filter chain is installed,
or that the resulting WAN path is secure. Neither daemon installation nor
firewall mutation is performed. Hardware gate must check `_NDM_FORWARD`,
Antiscan, native NDM access rules, rule regeneration and fail-closed behavior.
