# RouterForge Security Policy

RouterForge работает на роутере с root privileges и имеет privileged package/Management/DNS surfaces.

## Authentication
При включённой auth используется Entware `root`; password не сохраняется; session token in-memory; cookie `HttpOnly` + `SameSite=Strict`; failed-login tracking bounded/rate-limited.

Config: `/opt/etc/routerforge/security.json`. Invalid auth config fail closed.

## Network boundary
Core — единственный пользовательский RouterForge LAN listener `:2233`.
DNS/Admin/Monitoring используют root-owned Unix sockets.
Profiling — loopback-only `127.0.0.1:6061`.

## Management mutations
Process/service/file mutations требуют live root session, same-origin, confirmation/whitelist и Core-injected internal marker.

Browser не выбирает arbitrary terminal executable:
- Entware → fixed `/opt/bin/sh -il`;
- Keenetic → fixed server-resolved `ndmc`.

File Manager ограничен approved `/opt` + `/tmp` canonical path boundary.

## DNS mutations
Strict validation + snapshot → mutation → save → readback → verified rollback. Dynamic service/DHCP entries read-only.

## App Center / supply chain
Official lifecycle uses exact channel release-index, release asset URL, SHA256 and post-action version/state verification. Registry manifest не становится arbitrary shell script.

Stable promotion разрешён только из exact successful Dev FULL RELEASE artifact для того же SHA.

## Runtime Web UI Discovery
Нет blind LAN scan. Probe ограничен discovered local application endpoints и fail-closed redirect/XFO/CSP/SSRF checks.

## Reporting
Не публикуйте credentials, hashes, cookies/tokens, IP/MAC, internal domains, VPN keys или complete router configs.
