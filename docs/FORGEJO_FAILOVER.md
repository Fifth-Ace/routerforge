# Forgejo failover / failback control

RouterForge authority model:

- **GitHub = primary**
- **Forgejo = hot backup**
- normal replication is **GitHub -> Forgejo**
- **Forgejo -> GitHub is never automatic**

`scripts/routerforge-failover.ps1` changes only local Git authority/tracking. It never rewrites server refs.

## STATUS

`./scripts/routerforge-failover.ps1 -Mode STATUS`

Reads both remotes, prints main/dev parity and current local tracking. No mutation.

## PROMOTE_FORGEJO

Before promotion the external GitHub -> Forgejo sync job must be paused.

`./scripts/routerforge-failover.ps1 -Mode PROMOTE_FORGEJO -Confirm PROMOTE_FORGEJO -SyncPaused`

Promotion is fail-closed. Forgejo must be reachable and must match GitHub main/dev, or the last cached GitHub refs if GitHub is unavailable. Divergence stops promotion. The previous local tracking configuration is saved in `.git/routerforge-failover-state.json`; `remote.pushDefault`, `branch.dev.remote` and `branch.main.remote` are switched to Forgejo.

`-SyncPaused` is an explicit operator assertion that the server-side sync job must be paused first.

## Operation during failover

Do not use the dual `publish` remote while Forgejo contains commits not present on GitHub. Reverse synchronization is forbidden.

## RESTORE_GITHUB

`./scripts/routerforge-failover.ps1 -Mode RESTORE_GITHUB -Confirm RESTORE_GITHUB`

Both remotes must be reachable and exact main/dev parity is required. If Forgejo contains commits missing on GitHub, restore stops with **manual reconciliation** required. Forgejo -> GitHub is never automatic.

After successful restore, resume the normal GitHub -> Forgejo sync job externally.

## Safety contract

No force push, no reset-hard, no clean, no automatic reverse sync, no automatic conflict resolution, no server-side sync enable/disable.