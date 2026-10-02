# RouterForge P29 ecosystem pool — research snapshot 2026-10-02

Goal: build a broad Keenetic / Netcraze + Entware App Center pool while preserving each upstream project's own installation contract.

## Admission rule

RouterForge should not rewrite an upstream lifecycle merely to fit one installer model.

- Upstream OPKG feed -> structured/opkg.
- Upstream IPK/release asset -> release/IPK path.
- Upstream official shell installer -> official-script.
- Upstream git clone / host-side SSH installer / credential-heavy flow -> catalog entry first, automation only after a specific reviewed contract exists.
- No raw shell from unverified user sources.
- Every executable third-party action remains behind the App Center package-management/risk agreement.
- official-script downloads to a temporary file, records the SHA256 actually executed, invokes `sh <file> ...`, streams output, and never uses `curl | sh`.

## Already present in RouterForge catalog / audited overlay

| ID | Project | Current source | Current lifecycle |
| --- | --- | --- | --- |
| adguardhome-keenetic | Corvus-Malus/AdGuardHome-Keenetic | GitHub | preview/opkg |
| antiscan | dimon27254/antiscan | GitHub | preview official-script |
| awg-manager | hoaxisr/awg-manager | GitHub | preview official-script |
| bypass-keenetic | keenetic-dev/bypass_keenetic | GitHub | manual |
| chur-keenetic | ward-sentry/chur-keenetic | GitHub | manual |
| hydraroute-neo | Ground-Zerro/HydraRoute | GitHub + Keenetic forum | preview official-script |
| keen-pbr | maksimkurb/keen-pbr | GitHub | manual |
| keenetic-entware-extras | 0xkee/keenetic-entware-extras | GitHub + Keenetic forum | preview/manual |
| keenetic-sing-box-ui | CoOre/keenetic-sing-box-ui | GitHub | preview release-deploy |
| kvas | qzeleza/kvas | GitHub + Keenetic forum | preview official-script |
| nfqws | nfqws/nfqws-keenetic | GitHub | manual |
| nfqws-web | nfqws/nfqws-keenetic-web | GitHub | executable structured |
| nfqws2 | nfqws/nfqws2-keenetic | GitHub | executable structured |
| skeen | jinndi/SKeen | GitHub | preview official-script |
| traffic-via-vpn | rustrict/keenetic-traffic-via-vpn | GitHub | preview official-script |
| xkeen | Skrill0/XKeen | GitHub + Keenetic forum | preview official-script |
| xkeen-ui | zxc-rv/XKeen-UI | GitHub | preview official-script |
| keenetic-policy-ui | JohnDoe150489/keenetic-policy-ui | GitHub | executable structured |
| entware-manager | Di1r1/entware-manager | GitHub + Keenetic forum | preview release-deploy |
| zapret-gui | avatarDD/zapret-gui | GitHub | preview release-deploy |
| razvilka | ArtixSx/RAZVILKA | GitHub | preview official-script |
| keenetic-mcp | st412m/keenetic-mcp | GitHub | manual |

## New high-value candidates discovered

| Candidate | Upstream | Router relevance | Upstream lifecycle class | P29 direction |
| --- | --- | --- | --- | --- |
| B4 | DanielLavrushin/b4 | DPI bypass + Web UI, explicit Keenetic/Entware support | official install.sh | onboard as reviewed official-script |
| AntiGoblin | MaksimSamarin/AntiGoblin | Keenetic + Entware VPN routing panel | official install.sh | onboard as reviewed official-script |
| XKeen Panel | Dearonski/xkeen-panel | XKeen/Xray Web panel | official install.sh | onboard as reviewed official-script |
| XKeen UI (fan92rus) | fan92rus/xkeen-ui | alternate active XKeen UI | official setup.sh | separate catalog candidate after conflict audit |
| dropweb-xkeen | enkinvsh/dropweb-xkeen | Mihomo/XKeen VPN + Web panels | install.sh | reviewed official-script after conflict audit |
| TrustTunnel Keenetic | artemevsevev/TrustTunnel-Keenetic | TrustTunnel client + Keenetic hooks | interactive install.sh | catalog now; automate only reviewed noninteractive mode |
| TrustTunnel Keenetic Native | alex-combine/TrustTunnel-Keenetic-Native | native variant referenced by forum | release install.sh | candidate, audit fork relationship |
| Susanin.Keenetic | R17a/Susanin.Keenetic | adaptive VPN routing | install.sh; update/uninstall via local tool | strong candidate |
| MagiTrickle | MagiTrickle/MagiTrickle | DNS-driven selective routing + WebUI | repository bootstrap + opkg | structured/feed candidate |
| MagiTrickle mod (badigit) | badigit/MagiTrickle_mod_badigit | expanded routing, WebUI | official install.sh | reviewed official-script |
| MagiTrickle Mod (LarinIvan) | LarinIvan/MagiTrickle_Mod | alternate fork | add_repo.sh + opkg | candidate / conflict relation |
| KeenSnap | spatiumstas/KeenSnap | Keenetic backup to Telegram/WebDAV/GDrive | install.sh + feed | strong utility candidate |
| sms2gram | spatiumstas/sms2gram | modem SMS automation | install.sh + feed/opkg | strong utility candidate |
| web4static | spatiumstas/web4static | Web UI for many Keenetic Entware configs | install.sh + feed/opkg | strong administration candidate |
| tg-ws-proxy-go | spatiumstas/tg-ws-proxy-go | Telegram WS proxy for embedded/Keenetic | feed/opkg | structured feed candidate |
| tg-ws-proxy-rs | valnesfjord/tg-ws-proxy-rs | Telegram WS proxy with Entware installer | official install.sh | candidate |
| tg-ws-keenetic | Omn1z/tg-ws-keenetic | Telegram proxy + Web UI | git clone + install.sh | candidate, manual/assisted first |
| Bird4Static | DennoN-RUS/Bird4Static | policy/BGP/static routing | git clone + interactive install.sh | catalog/manual first |
| IPset4Static | DennoN-RUS/IPset4Static | DNS/IPSet routing addon | project scripts | catalog/manual first |
| keenetic-vpn-mihomo | dd/keenetic-vpn-mihomo | selected-device Mihomo VPN | host-side SSH installer | catalog/manual first; cannot run router-local unchanged |
| AIWAY Manager | kirniy/aiway | Keenetic SNI/DNS manager | router install.sh | candidate reviewed official-script |
| WDTT Server Entware | kkvoru/wdtt-server-entware | router tunnel server | release install.sh + checksums | candidate, interactive secrets |
| Netcraze Giga AWG 3.1 | Sergekkk/netcraze-giga-awg3.1 | hardware-specific AWG kernel integration | copied installer | detect/manual due device specificity |
| AdGuard Home upstream | AdguardTeam/AdGuardHome | widely used on Keenetic forum | release binary | source candidate; RouterForge already carries Keenetic-specific wrapper |
| web4core | spatiumstas/web4core | Xray/sing-box/Mihomo config generator | web/config utility | catalog candidate after runtime audit |
| MagiTrickle CLI | MagiTrickle/MagiTrickle-cli | CLI companion | package/tool | companion candidate |
| Keenetic Entware Extras: geo-split | 0xkee/keenetic-entware-extras | GeoIP/domain split routing | feed/opkg | expose as child package candidate |
| Keenetic Entware Extras: webui | 0xkee/keenetic-entware-extras | Web panel | feed/opkg | expose as child package candidate |
| Keenetic Entware Extras: smartdns-geo-conf | 0xkee/keenetic-entware-extras | SmartDNS geo routing | feed/opkg | child package candidate |
| Keenetic Entware Extras: smartdns-redirect | 0xkee/keenetic-entware-extras | SmartDNS redirect helper | feed/opkg | child package candidate |
| Keenetic Entware Extras: net-check | 0xkee/keenetic-entware-extras | tunnel/network quality checks | feed/opkg | child package candidate |
| WireGuard DPI bypass | Ground-Zerro/Wireguard-DPI-blocking-bypass | Keenetic WireGuard helper | scripts | audit candidate |
| tg-ws-proxy upstream family | Flowseal/tg-ws-proxy | Telegram WS proxy family | platform-dependent | source/family candidate |

## Source URLs used in this sweep

GitHub:
- https://github.com/Di1r1/entware-manager
- https://github.com/fan92rus/xkeen-ui
- https://github.com/Dearonski/xkeen-panel
- https://github.com/CoOre/keenetic-sing-box-ui
- https://github.com/enkinvsh/dropweb-xkeen
- https://github.com/DanielLavrushin/b4
- https://github.com/MaksimSamarin/AntiGoblin
- https://github.com/artemevsevev/TrustTunnel-Keenetic
- https://github.com/R17a/Susanin.Keenetic
- https://github.com/MagiTrickle/MagiTrickle
- https://github.com/badigit/MagiTrickle_mod_badigit
- https://github.com/LarinIvan/MagiTrickle_Mod
- https://github.com/spatiumstas/KeenSnap
- https://github.com/spatiumstas/sms2gram
- https://github.com/spatiumstas/web4static
- https://github.com/spatiumstas/tg-ws-proxy-go
- https://github.com/valnesfjord/tg-ws-proxy-rs
- https://github.com/Omn1z/tg-ws-keenetic
- https://github.com/DennoN-RUS/Bird4Static
- https://github.com/dd/keenetic-vpn-mihomo
- https://github.com/kirniy/aiway
- https://github.com/kkvoru/wdtt-server-entware
- https://github.com/Sergekkk/netcraze-giga-awg3.1
- https://github.com/Corvus-Malus/AdGuardHome-Keenetic
- https://github.com/0xkee/keenetic-entware-extras
- https://github.com/Ground-Zerro/HydraRoute

Keenetic Community threads:
- Entware Manager
- RouterForge
- HydraRoute Neo
- xKeen / AWG / HydraRoute discussion
- geo split routing / keenetic-entware-extras
- Susanin.Keenetic
- TrustTunnel on Keenetic
- MagiTrickle
- KeenSnap
- AdGuardHome
- Bird4Static

## Next batches

P29-1: migrate the five P27 audited overlay entries into canonical registry metadata.

P29-2: turn already-audited official-script entries into executable actions where upstream has a deterministic router-local command.

P29-3+: ingest new candidates in small reviewed batches grouped by lifecycle model:
1. feed/opkg;
2. noninteractive official-script;
3. interactive/secret-bearing/manual;
4. host-side installers and hardware-specific projects.

Do not enable executable lifecycle merely because a repo was discovered. Discovery expands the catalog pool; execution authority comes only from a reviewed lifecycle contract.
