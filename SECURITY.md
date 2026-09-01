# Security Policy

## Reporting a vulnerability

Please do **not** publish credentials, public IP addresses, MAC addresses,
private hostnames, tunnel keys or complete router configuration dumps in a
public issue.

For a suspected security issue, open a minimal GitHub issue without sensitive
details and indicate that additional diagnostics are available privately.

When attaching normal diagnostics, sanitize:

- public IP addresses;
- MAC addresses;
- private device/host names;
- internal domains;
- VPN/WireGuard/AmneziaWG keys and credentials;
- tokens, passwords and cookies.

DNS Monitor performs passive packet capture and requires root privileges on the
router, so only install builds obtained from this repository or builds you have
compiled yourself.
