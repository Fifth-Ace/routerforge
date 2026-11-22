#!/usr/bin/env python3
"""Read-only NFQWS Svelte source/contract gates; browser parity remains separate."""
from pathlib import Path
import hashlib
import re
import subprocess

r=Path(__file__).resolve().parents[1]
f=r/'modules/nfqws-manager/frontend'
h=(f/'index.html').read_text(encoding='utf-8')
s=(f/'NFQWSManagerApp.svelte').read_text(encoding='utf-8')
j=(f/'NfqwsRuntime.js').read_text(encoding='utf-8')
c=(f/'module.css').read_text(encoding='utf-8')
entry=(f/'main.js').read_text(encoding='utf-8')
markup=s.split('</script>',1)[1]
ids=re.findall(r'\bid="([A-Za-z0-9:_-]+)"',markup)
assert len(ids)==626 and len(set(ids))==626, 'NFQWS_DOM_IDS_DROPPED_OR_DUPLICATED'
assert hashlib.sha256('\n'.join(ids).encode()).hexdigest() == 'b0593cb1011b73243ba717f03d554444bc7ccf940b0690a00b5ac15343a3201c', 'NFQWS_LEGACY_DOM_ID_PARITY_MISMATCH'
assert len(h)<2000 and '<script defer src="svelte-mode.js"></script>' in h
assert '<script>' not in h and '<style>' not in h and 'NfqwsRuntime' not in h
assert len(c)>120000 and len(s)>90000 and len(j)>300000, 'NFQWS_SOURCE_TRUNCATED'
assert 'mount(NFQWSManagerApp' in entry and "from 'svelte'" in entry
assert "onMount(() =>" in s and 'startNFQWSRuntime()' in s
assert 'class:active={activeTab' in s and "new CustomEvent('routerforge:nfqws:tab'" in j
assert j.count('function ')>=450, 'NFQWS_LEGACY_BEHAVIOR_TRUNCATED'
# Do not lose exact SHA / confirmation / rollback requirements when moving code.
for needle in ('expected_sha256:listMeta.sha256', 'expected_sha256:analysisHashes[target]',
               'if(!result.post_apply_verified)', 'expected_config_changes:p.config_changes',
               "confirm:'ROUTERFORGE_LIST_RENAME_APPLY'", "confirm:'NFQWS_LIST_DELETE'",
               'result.regression_guards_verified'):
    assert needle in j, 'NFQWS_SAFETY_MARKER_MISSING: '+needle
for filename in ('main.js','NfqwsRuntime.js'):
    subprocess.run(['node','--check',str(f/filename)],check=True)
print('NFQWS_SVELTE5_SOURCE: PASS / 626 exact IDs / native nav / guard contracts / syntax')
print('NFQWS_FULL_NATIVE_OWNERSHIP: PENDING / legacy DOM controller present')
