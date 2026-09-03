# Marketplace research snapshot — 2026-09

Curated upstream repositories reviewed for the combat-preview catalog:

- https://github.com/hoaxisr/awg-manager
- https://github.com/nfqws/nfqws-keenetic
- https://github.com/nfqws/nfqws2-keenetic
- https://github.com/nfqws/nfqws-keenetic-web
- https://github.com/Ground-Zerro/HydraRoute
- https://github.com/Skrill0/XKeen
- https://github.com/zxc-rv/XKeen-UI
- https://github.com/maksimkurb/keen-pbr
- https://github.com/jinndi/SKeen
- https://github.com/qzeleza/kvas
- https://github.com/keenetic-dev/bypass_keenetic
- https://github.com/rustrict/keenetic-traffic-via-vpn
- https://github.com/Corvus-Malus/AdGuardHome-Keenetic
- https://github.com/ward-sentry/chur-keenetic
- https://github.com/CoOre/keenetic-sing-box-ui
- https://github.com/0xkee/keenetic-entware-extras

## Selection

The list is intentionally curated rather than trying to mirror every Entware
package. Priority is given to projects that are visible in the Keenetic/Netcraze
community, actively useful on modern Entware installations and documented well
enough to build deterministic detection metadata.

Popularity counts are not rendered in the runtime UI because they change over
time. At this research snapshot, particularly visible projects include the
nfqws family, AWG Manager, XKeen/XKeen-UI, keen-pbr, bypass_keenetic and
keenetic-traffic-via-vpn.

## Verified metadata highlights

- AWG Manager: package `awg-manager`, service `S99awg-manager`, UI normally
  `:2222`, official HTTPS GitHub installer also exists.
- nfqws2: package `nfqws2-keenetic`; its feed declares a conflict with legacy
  `nfqws-keenetic`.
- nfqws Web UI: package `nfqws-keenetic-web`, official all-arch Entware feed,
  UI `:90`; supports both nfqws and nfqws2.
- XKeen: official installer downloads the current `xkeen.tar` release and runs
  `xkeen -i`.
- XKeen UI: service `S99xkeen-ui`, binary `/opt/sbin/xkeen-ui`, UI `:1000`.
- keen-pbr: current 3.x releases publish dedicated Keenetic ARM64 IPKs; full
  build includes an optional web UI/API and a headless package is also
  available.
- SKeen: service `S99SKeen`, config under `/opt/etc/skeen`, built-in dashboard
  defaults to `:9999`.
- AdGuard Home: Entware package `adguardhome-go`, service `S99adguardhome`,
  initial UI `:3000`; Keenetic `dns-override` remains a separate explicit
  ownership-changing step.
- Chur Keenetic: package `chur-keenetic`, service `S99chur-keenetic`, ARM64
  feed available, UI `:8088`.
- Keenetic sing-box UI: service `S99keenetic-sing-box-ui`, UI `:9091`;
  upstream installs from a host/release workflow rather than an Entware feed.
- Keenetic Entware Extras: package suite with an optional WebUI on `:8080`.

## Safety rules

- Third-party binaries and IPKs are never mirrored into the DNS Monitor
  repository.
- Every external install plan is `preview_only`.
- HTTP installer URLs are shown with warnings and never executed
  automatically.
- DNS ownership, firewall, TProxy, iptables/ipset and routing mutations remain
  manual until a transactional authenticated installer exists.
- Ambiguous WebUI ports are not guessed.
- A detected manually-installed project stays external; DNS Monitor does not
  silently assume ownership.
