# RouterForge Telemetry Collector (T2)

Minimal Python 3.13 stdlib HTTP collector, SQLite WAL, admin Basic-auth dashboard. No external Python packages. Designed to run **behind a TLS-terminating reverse proxy**, bound on host loopback only. Do not publish container port or HTTP directly. This is a staging-ready backend, **not deployed**.

- Public (through HTTPS proxy): `POST /v1/events` with allowlisted schema v1, max 2048 bytes.
- Private: `GET /admin` with HTTP Basic auth over HTTPS **plus** an administrator-only access boundary (VPN or IP allowlist). Reverse proxy must not publicly expose `/admin` or `/healthz`.
- Local probe: `GET /healthz`.
- Storage: `/data/telemetry.sqlite`, WAL enabled.
- Distinct registrations keyed by random 128-bit installation ID; retries deduped by `event_id`. No uninstall tracking.
- This service does not log IPs, payloads, or installation IDs. Ensure reverse-proxy, infrastructure and WAF logs follow the same privacy policy. Remote IPs are still processed by the network stack.
- Unauthenticated `/v1/events` means untrusted clients **can forge registrations**. Add proxy rate limits and anomaly alerting; never claim cryptographically verified physical installs.
- No installed device should point here until an HTTPS FQDN and privacy notice are configured and verified.

Testing: `python -m unittest discover -s tests -v` from the collector folder.

Staging with Compose, from `deploy/`: create a `.env` containing a unique >=20-character `RF_ADMIN_PASSWORD`, then `docker compose up --build -d`. The bind remains `127.0.0.1:18081`; do not change existing proxy services or firewall. `curl http://127.0.0.1:18081/healthz` should return `{"ok":true}`. The host reverse proxy can route only `/v1/events` to this address after explicit deployment approval.

Suggested HTTPS setup: a **new dedicated hostname**, upstream `http://127.0.0.1:18081`, public path `/v1/events` only, private `/admin` via VPN, no access logging, rate limits, body size 2 KiB, standard TLS verification. TLS termination is *not* in this repository. The actual domain must be selected before Core sends any events.

Maintenance: backup the volume with consistent SQLite backup tools, avoid copying live WAL database files separately. Short-lived event-id dedupe retention can be enabled once a 45-day retry maximum is enforced. Stale raw dedupe records currently remain; their disk usage must be monitored or a timed prune job deployed with retention guarantees.
