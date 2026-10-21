# RouterForge Telemetry — distribution analytics (schema v2)

RouterForge Core reports **installation registration and daily presence** separately
from **optional technical telemetry**. Technical telemetry defaults to enabled
and can be disabled at any time under Settings → RouterForge statistics.
Disabling telemetry does not affect installation registration, presence or
local router metrics. The project does not count uninstalls.

## Basic accounting (not disabled by the diagnostic toggle)

- Random 128-bit installation ID, stable across package updates; random event ID
  for idempotent retry (both are pseudonymous identifiers).
- Core binary version and **actual installed build channel** (`stable`, `beta`,
  `dev`), inferred from that binary's version, **not** the selected update feed.
- Registration once (on first confirmed server receipt), daily presence thereafter.
  Collector counts distinct IDs; installs that never reach it cannot be counted.

## Optional telemetry (default on)

- Installed CPU architecture (`arm64`, `mips`, `mipsel`, `unknown`).
- A bounded allowlist of official installed RouterForge module IDs and **installed
  OPKG versions**. Uninstalled modules are absent from each replacement snapshot.
- Core version and release channel are repeated in the optional report.
- **Not sent:** CPU load, RAM usage, temperature, a synthetic health status,
  DNS traffic, domain names, IP/MAC addresses as payload fields, credentials,
  customer packages, subscriptions, serial number, hostname or log contents.

The HTTPS receiver still processes source network IPs; do not claim network
anonymity. Default app logging does not store source IPs or payloads; ensure the
reverse proxy is configured consistently.

## Wire protocol / backwards compatibility

- `POST https://telemetry.fifthace.ru/v1/events`, authenticated only by transport
  encryption; treat incoming installations as **unverified self-reports**.
- **v2**: `register` and `presence` require `core_version` + `release_channel`;
  `diagnostic` requires those, `architecture` and `modules` list.
- Existing **v1** registers, presence and diagnostics remain accepted by Collector
  during rollout. They do not provide reliable current build channels or module
  snapshots; those are marked as `unknown` until v2 arrives.
- Schema allowlists on both ends; event IDs deduplicate retries; no bulk logging.
- The server migrates existing SQLite records **in-place** and preserves existing
  counts. Validate on a copied DB before production deployment and take a
  consistent SQLite backup with WAL handling. **Do not delete the Docker volume.**
- Module counts use **distinct installation IDs** with successful optional
  snapshots in the last 7 days; show denominator / coverage explicitly.
- Core-channel and version views use the last 30 days of presence for freshness.
- Physical online state and uninstalls cannot be inferred reliably.

## Rollout order

1. Merge/CI on `dev` only; a successful push updates rolling ARM64 Dev automatically.
2. Independently test and deploy schema-v2 Collector on the VPS with a real DB
   backup and smoke tests. Reverse proxy, xray and proxy ports remain unchanged.
3. Run one separately approved test-device Core install and verify one registration,
   repeated presence without a duplicate, channel and module versions in `/admin`.
4. Only afterward consider preparing a later Beta. Published Beta 9 untouched.

Until step 2 completes, newly upgraded Dev Core can receive HTTP 400 responses
for v2 events from the old collector; the client **retries with persistent event
IDs** and must never block the router. All earlier v1 clients remain compatible
with the upgraded Collector. Never mistake CI PASS for a live VPS deploy.
