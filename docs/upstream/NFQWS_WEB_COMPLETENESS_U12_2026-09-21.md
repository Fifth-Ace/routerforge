# RouterForge U12 — NFQWS Web Completeness Audit + Gap Closure

Base SHA: `98144c964aaa7eca783f3577465436f76754da79`

Static route/UI audit identified eleven current NFQWS runtime endpoints without a
direct frontend reference at the U11 baseline.

Closed as user-facing gaps in U12:
- `/v2/memory` + `/v2/memory/clear`: Target Memory browser, policy/confidence
  display, explicit per-target/all clear.
- `/v2/nfqueue-health`: observation-only NFQUEUE process/firewall/kernel view.
- `/v2/bench-profiles`: runtime-advertised transport capability view.
- `/backups/prune`: explicit retention prune from Backup Center.
- `/clienthello/validate`: validate installed blobs from Blob Manager.
- `/v2/geo/source`: edit provenance metadata for an existing Geo asset.

Classified as implementation/superseded routes rather than missing user workflows:
- `/dpi-detector/run`, `/dpi-detector/v5/run`, `/dpi-detector/v5/console`:
  superseded by current v5 streaming endpoint and native PTY/TUI workflow.
- `/v2/selector-progressive`: lower-level selector implementation route; the
  supported web workflow uses `/v2/selector` with planner/progress/live verify.
- `/clienthello/validate` is now surfaced, but generate already validates generated
  bytes internally; the new UI action is specifically for installed blobs.

Safety:
- Runtime health is read-only and never repairs/restarts anything.
- Target Memory clear changes planner history only, never production config/runtime.
- Backup prune deletes snapshots only according to backend retention policy.
- Geo provenance updates metadata only.
- ClientHello validation is read-only.
- No new backend mutation authority is introduced.
