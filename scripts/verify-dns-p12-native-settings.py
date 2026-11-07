#!/usr/bin/env python3
"""Fail-closed source safety checks for Svelte-native DNS Lite Settings."""
from pathlib import Path
import re, sys
r=Path(__file__).resolve().parents[1]
f=r/'modules/dns-router-lite/frontend'
s=(f/'DnsP25Settings.svelte').read_text(encoding='utf-8')
main=(f/'main.js').read_text(encoding='utf-8')
test=(r/'modules/dns-router-lite/frontend-tests/native-bundle.test.cjs').read_text(encoding='utf-8')
checks={
    'Svelte-native Settings state and real markup':all(t in s for t in ("let active = $state(false)","let section = $state('service')","let ipRows = $state([])","{#each sections as item","id=\"p25SettingsCard\"","data-settings-section={item.id}")),
    'all eight Settings sections rendered':len(re.findall(r'id="p25Settings_(?:service|cache|ip|upstream|security|privacy|dns64|performance)"',s))==8,
    'P28 through P31 editor fields':all(('id="'+t+'"') in s for t in ('p28Enabled','p28Mode','p28Timeout','p29Enabled','p29MinTTL','p29MaxTTL','p29Concurrent','p29Optimistic','p29Pending','p30Enabled','p30ECSMode','p30ECSAddress','p30DNSSECMode','p31DNS64Enabled','p31DNS64Prefix','p31NAT64Confirmed','p31ReverseEnabled','p31ReverseUpstream')),
    'read-only diagnostics with bounded lifetime':all(t in s for t in ("method:'GET'","ipRows=monitorRowsFrom(data)","clearInterval(timer)","observer?.disconnect()","document.hidden")),
    'old editor state kept when mode is switched':all(t in s for t in ('returnTo(cacheField,originalCache)','returnTo(additional,originalAdditional)','if(!advanced) return;')),
    'all saved DNS mutations remain in guarded controller':all(t in (f/'DnsLiteController.svelte').read_text(encoding='utf-8') for t in ('If-Match',"on('gateEnable'","on('gateDisable'","on('commitDNS'")),
    'no source-compatibility imperative Settings':not any(t in s for t in ('RF_ACTUAL_CONTROLLER_START','document.createElement','replaceChildren(','.innerHTML','document.write(',"method:'POST'","method:'PUT'",'XMLHttpRequest')),
    'bundle mount remains Svelte-owned':"import DnsP25Settings from './DnsP25Settings.svelte'" in main and "[DnsLiteController,DnsP22Visual,DnsP25Settings]" in main,
    'real-bundle browser smoke covers Settings':all(t in test for t in ('p25SettingsNav','data-settings-section="privacy"','p22ModeBasic','originalCacheParent','method===\'GET\'')),
}
for name,ok in checks.items():print(('PASS' if ok else 'FAIL')+': '+name,flush=True)
if not all(checks.values()):raise SystemExit('NATIVE_SETTINGS_SOURCE_GATE: FAIL')
print('DNS_LITE_P12_NATIVE_SETTINGS_SOURCE_GATE: PASS')
