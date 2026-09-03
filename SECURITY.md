# RouterForge Security Policy

RouterForge runs on a router with root privileges, observes network/DNS state and can perform approved package lifecycle operations. Treat its web access and release supply chain accordingly.

## Authentication

Authentication is optional and configurable in RouterForge Settings.

When enabled:

- only Entware user `root` is accepted;
- password verification uses `/opt/etc/shadow`, with `/opt/etc/passwd` fallback;
- the password is not stored by RouterForge;
- sessions use a random in-memory token;
- session lifetime is 12 hours;
- cookie is `HttpOnly` and `SameSite=Strict`;
- same-origin checks protect auth-changing POST requests;
- repeated failed logins are rate-limited.

Config:

```text
/opt/etc/routerforge/security.json
```

If the config exists but cannot be read or parsed correctly, Core is designed to fail closed for authentication state.

## API boundary

When auth is required, `/api/*` is protected except the explicit authentication endpoints and `/api/health`.

Do not expose port `2233` to untrusted networks without authentication and appropriate network filtering.

## Module boundary

Monitoring helpers and RouterForge Control communicate over root-owned Unix sockets.

RouterForge Control is read-only.

Helpers should not open independent LAN web ports.

## Marketplace / packages

Official RouterForge package updates:

- use the current channel release-index;
- use exact release asset URLs;
- require a matching SHA256 before `opkg install`;
- restrict RouterForge downloads to project GitHub release HTTPS URLs;
- use supported typed lifecycle operations.

Registry manifests are not arbitrary shell scripts.

Third-party entries do not automatically become trusted merely because they are detected.

## Profiling

Profiling is loopback-only by design.

Default:

```text
127.0.0.1:6061
```

Use SSH forwarding for remote profiling.

## Reporting a vulnerability

Do not publish sensitive details in a public issue.

Open a minimal issue stating that a security problem exists and that private diagnostics are available.

Sanitize:

- credentials and password hashes;
- cookies/tokens;
- public IP addresses;
- MAC addresses;
- private host/device names;
- internal domains;
- WireGuard/AmneziaWG/VPN keys;
- complete router configuration dumps.

## Release provenance

Use packages from the RouterForge GitHub releases or builds you compiled yourself.

Stable:

```text
routerforge-stable
```

Beta:

```text
routerforge-beta
```

Do not install IPKs received through unrelated mirrors unless you independently verify their provenance and checksum.
