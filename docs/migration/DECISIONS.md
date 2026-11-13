# Decisions / security invariants

- Exact source base must match origin/dev; SHA drift stops, never overlay stale source.
- Svelte is the only owner of any UI migrated in this slice; business API calls remain in explicit command modules temporarily. Never delete legacy code before behavioral parity.
- No backend, Keenetic, ndnproxy, stubby, Entware, RF gate or firewall modification.
- No implicit apply/start on mount. Persisted changes require explicit user action, CAS / If-Match, protected confirmations, double-click lock and safe rollback.
- Backup Center Restore/Delete/Prune, list installs and Smart Apply keep existing server-side protections; UI conversion must not auto-commit.
- Module DONE is not a metric of Svelte files; needs active legacy eliminated, real browser/Go/IPK/upgrade evidence.
- Git actions, nonpublishing CI dispatch, publication and router installation require separate explicit authorization.

## NFQWS raw active config owner (2026-11-11)
- New Svelte owner replaces active-config DOM renderer/controller without modifying backend or system service.
- Keep existing backup/reload/rollback contract. A read-before-write SHA preflight is only client-side; it must never be represented as atomic server CAS. Any backend CAS/If-Match retrofit needs separate authorization.
- Full MODULE DONE remains pending until all other active NFQWS screens have one Svelte owner and required behavioral/IPK gates are executed.
