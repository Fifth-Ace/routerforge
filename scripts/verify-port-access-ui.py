#!/usr/bin/env python3
"""P7: Port Access Svelte 5 source, safety, packaging and UI contract gates.

Source inspection, not an automated browser or live firewall acceptance test.
"""
from html.parser import HTMLParser
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
FRONT = ROOT / 'modules/port-access-manager/frontend'
HTML = FRONT / 'index.html'
BUILD = ROOT / 'scripts/build-port-access-manager-opkg.sh'

class DOM(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = {}
        self.scripts = []
    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if a.get('id'):
            self.ids.setdefault(a['id'], []).append(tag)
        if tag == 'script' and a.get('src'):
            self.scripts.append(a['src'])

def main():
    failures = []
    def gate(label, condition):
        print(('PASS' if condition else 'FAIL') + ': ' + label, flush=True)
        if not condition:
            failures.append(label)
    html = HTML.read_text(encoding='utf-8')
    build = BUILD.read_text(encoding='utf-8')
    dom = DOM()
    dom.feed(html)
    gate('single compiled Svelte entry', dom.scripts == ['svelte-status.js'])
    gate('no duplicate static IDs', all(len(tags) == 1 for tags in dom.ids.values()))
    gate('no browser-side forms capable of accidental submit', not re.search(r'<form\b', html, re.I))
    names = ['status','engineering','engine-workspace','odin','log','layout']
    gate('single Svelte root target', dom.ids.get('pa-svelte-layout') == ['div'] and
         all(('pa-svelte-' + n) not in dom.ids for n in names if n != 'layout'))
    gate('required CSS remains linked', all(name in html for name in
         ('module.css','ui-rebuild.css','p39-compact.css','p53-ux.css','p54-workspace.css','p55-antiscan.css')))
    old = ['app.js','odin-ui.js','operation-log.js','engine-wizard.js','config-editor.js',
           'ui-rebuild.js','p39-compact.js','p53-ux.js','p54-workspace.js','p55-antiscan.js']
    gate('no legacy JS source', all(not (FRONT / name).exists() for name in old))
    gate('no legacy JS packaged', all(name not in build.split('for file in ',1)[1].split('; do',1)[0] for name in old))
    gate('explicit IPK legacy deny gate', 'for legacy in' in build and
         all(name in build.split('for legacy in',1)[1].split('; do',1)[0] for name in old))
    gate('compiled Svelte IPK asset', 'npm run build' in build and
         'build/svelte-status.js' in build and 'ui/svelte-status.js' in build)
    entry = (FRONT / 'status-main.js').read_text(encoding='utf-8')
    component_names = ['PortAccessStatus','PortAccessOperationLog','PortAccessEngineWorkspace',
                       'PortAccessOdin','PortAccessEngineering','PortAccessLayout']
    gate('only one Svelte mount', entry.count('mount(') == 1 and
         'mount(PortAccessLayout' in entry and not any('mount(' + name in entry for name in component_names if name != 'PortAccessLayout'))
    source = {name:(FRONT / (name + '.svelte')).read_text(encoding='utf-8') for name in component_names}
    gate('all components use reactive Svelte 5 state', all('$state(' in s for s in source.values()))
    layout = source['PortAccessLayout']
    gate('Svelte owns four pages and three protection methods', all(x in layout for x in
         ("'protection'","'access'","'engines'","'diagnostics'","'odin'","'knockd'","'fwknopd'")))
    gate('direct Svelte engine selection, no synthetic events',
         'selectedEngine={' in layout and 'onEngineChanged=' in layout and
         "new Event('change'" not in layout and '.dispatchEvent(' not in layout)
    gate('all five views are direct Svelte children, no mounted DOM relocation',
         all('<' + name + ' ' in layout or '<' + name + ' />' in layout for name in component_names if name != 'PortAccessLayout') and
         all('pa-svelte-' + name in layout for name in names if name != 'layout') and
         '.appendChild(' not in layout)
    gate('state crosses component boundary by callbacks, never DOM click bridge',
         all(name in layout for name in ('onServiceState=', 'onDeployState=', 'onPreviewReady=', 'onPlanInvalidated=', 'onStateChanged=')) and
         '.click()' not in layout and 'rfpa:service-state' not in layout)
    gate('Svelte reports state from live read-only API',
         '/api/modules/port-access-manager/odin' in layout and '/engine-service?engine=' in layout)
    gate('Svelte KKD list is fail closed',
         "data.list !== 'KKD'" in layout and 'Array.isArray(data.entries)' in layout)
    gate('Svelte cleans timers and listeners', 'window.clearInterval' in layout and
         'document.removeEventListener' in layout)
    gate('read-only preflight stays guarded', 'mutation_api !== false' in source['PortAccessStatus'] and
         'ready_for_apply !== false' in source['PortAccessStatus'])
    gate('operation log bound, bounded and clean',
         'data.scope !== \'operations\'' in source['PortAccessOperationLog'] and
         'window.clearInterval(interval)' in source['PortAccessOperationLog'])
    engine = source['PortAccessEngineWorkspace']
    gate('Entware safety, CAS and watchdog intact', all(x in engine for x in
         ('base_sha256','RESTORE_CONFIG','SAVE_CONFIG','credential_once',
          'replace_existing','onServiceState','onDeployState','onPreviewReady','watchdog')))
    gate('Entware action IDs owned by Svelte', all('id="' + x + '"' in engine for x in
         ('engine-config-engine','engine-wizard-preview','rfpa-prepare','rfpa-enable',
          'rfpa-confirm','rfpa-disable','rfpa-refresh','existing-engine-control')))
    odin = source['PortAccessOdin']
    gate('odin watchdog and server verification retained', all(x in odin for x in
         ('watchdog_deadline_unix','server_unix','confirm','window.confirm',
          'onStateChanged')))
    engineering = source['PortAccessEngineering']
    gate('engineering denies unintended firewall mutation', all(x in engineering for x in
         ('firewall_mutation !== false','deployment_blocked !== true','confirm:\'REVOKE\'')))
    gate('no eval/document.write in native UI', all('eval(' not in s and 'document.write(' not in s
         for s in source.values()))
    print('P40A_UI_CONTRACT: ' + ('FAIL: ' + ', '.join(failures) if failures else 'PASS'), flush=True)
    return 1 if failures else 0

if __name__ == '__main__':
    sys.exit(main())
