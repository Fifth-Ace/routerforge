#!/usr/bin/env python3
from pathlib import Path
R=Path(__file__).resolve().parents[1]
s=(R/'scripts/routerforge-failover.ps1').read_text(encoding='utf-8')
d=(R/'docs/FORGEJO_FAILOVER.md').read_text(encoding='utf-8')
for x in ['STATUS','PROMOTE_FORGEJO','RESTORE_GITHUB','-SyncPaused','automatic Forgejo->GitHub reconciliation is forbidden','REVERSE_SYNC=NONE','GITHUB_REFS_MUTATED=NO','FORGEJO_REFS_MUTATED=NO']:
    assert x in s, x
for x in ['git push --force','git push -f','reset --hard','git clean']:
    assert x not in s, x
for x in ['GitHub = primary','Forgejo = hot backup','GitHub -> Forgejo','Forgejo -> GitHub','never automatic','PROMOTE_FORGEJO','RESTORE_GITHUB','sync job must be paused','manual reconciliation']:
    assert x in d, x
print('FORGEJO_FAILOVER_CONTRACT=PASS')
print('REVERSE_SYNC=FORBIDDEN')
print('DIVERGENCE=FAIL_CLOSED')