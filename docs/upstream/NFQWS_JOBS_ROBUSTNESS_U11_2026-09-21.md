# RouterForge U11 — Jobs / Maintenance / Robustness

Base SHA: `6e0ccd631f1e0fe42cd4a71d5943b511f9670b91`

This stage extends the existing RouterForge Maintenance scheduler at
`/opt/etc/routerforge/nfqws-jobs.json`; it does not introduce a second scheduler.

Supported automatic jobs:
- detect-target;
- tcp16-revalidate;
- strategy-health-recheck;
- nfqueue-health;
- backup-cleanup.

`list-refresh` is explicitly advertised as unavailable until RouterForge has a
refresh-capable source provider with a pinned/validated origin. It fails closed.

Robustness:
- service restart does not trigger an immediate job storm;
- first/periodic cadence is reported separately;
- one bounded retry for transport/5xx failures only;
- single-flight is enforced by kind+target across different job IDs;
- OK / INCONCLUSIVE / FAILED / UNHEALTHY are separate outcomes;
- last-known-good output/status/time survive later failures;
- detect/strategy jobs reject results if production config SHA changes while they run;
- TCP16 carries exact expected production config SHA;
- NFQUEUE health is observation-only and never repairs/restarts production;
- backup cleanup prunes only existing backup stores.

No job edits nfqws2.conf, firewall rules or restarts production nfqws2.
