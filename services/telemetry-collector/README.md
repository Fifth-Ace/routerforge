# RouterForge Collector — schema v2

Python 3.13 stdlib and SQLite WAL. Loopback Docker port `127.0.0.1:18081:8000`.
Keep HTTPS termination in the existing reverse proxy, public `/v1/events` only.
Keep `/admin` and `/healthz` private; /admin requires Basic Auth *and* a network
access boundary. See `docs/TELEMETRY.md` for exactly collected fields.

**Important: Deploy independently of Core CI.** Code being merged into dev does
NOT update the running VPS. Do not overwrite/replace the named Docker volume.

Safe deployment checklist on the VPS (do not skip):

1. Confirm current container, mount/volume name and Compose working directory.
2. Quiesce writers or use `sqlite3.Connection.backup` for a consistent database
   copy, retain backup off-container and verify it opens.
3. Run Python collector tests and migration against a COPY of the real database,
   confirm registered_total unchanged and legacy v1 accepted.
4. Build/recreate only the collector service with the existing Compose project,
   unchanged reverse proxy and TLS. Verify local `/healthz` on loopback.
5. Using nonproduction IDs on a separate test database, verify v1 and v2
   registration, no duplicate counts, module snapshot replacement and private
   dashboard authorization; do not inject fake installs into production.
6. Back up and retain the old image and migration-compatible DB copy for rollback.
   If rollback to v1 is required, restore the old DB backup as v1 cannot read new
   schema-v2 events; old server rejects v2 until Collector is updated again.

`python3 -m unittest discover -s tests -v` tests protocol, migration and dashboard.
The service suppresses application request logs; the reverse proxy must also
avoid storing installation IDs and source IP data unnecessarily. `/admin` must
never be anonymously publicly reachable. No separate analytics stack required.
