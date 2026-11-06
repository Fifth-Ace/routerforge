#!/usr/bin/env python3
"""P40A: regression contracts for the shipped Port Access Manager UI.

Static DOM/wiring checks, not a browser or real firewall acceptance test.
"""
from html.parser import HTMLParser
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
FRONT = ROOT / 'modules/port-access-manager/frontend'
HTML = FRONT / 'index.html'
BUILD = ROOT / 'scripts/build-port-access-manager-opkg.sh'

class DOM(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = {}
        self.scripts = []
        self.buttons = {}
    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if 'id' in a:
            id_ = a['id']
            self.ids.setdefault(id_, []).append((tag, a))
        if tag == 'script' and 'src' in a:
            self.scripts.append(a['src'])
        if tag == 'button' and 'id' in a:
            self.buttons[a['id']] = a

def main():
    failures = []
    def check(condition, label):
        print(('PASS' if condition else 'FAIL') + ': ' + label)
        if not condition:
            failures.append(label)
    dom = DOM()
    dom.feed(HTML.read_text(encoding='utf-8'))
    required_ids = [
        'engine-config-engine','engine-config-panel','engine-config-editor',
        'engine-wizard-panel','engine-wizard-interface','engine-wizard-knock',
        'engine-wizard-window','engine-wizard-ttl','engine-wizard-services',
        'engine-wizard-preview','engine-wizard-draft-save','engine-wizard-draft-load',
        'engine-wizard-result','engine-wizard-plan','rfpa-deploy-panel',
        'rfpa-prepare','rfpa-enable','rfpa-confirm','rfpa-disable','rfpa-refresh',
        'rfpa-replace-existing','rfpa-deploy-status','rfpa-credentials',
        'rfpa-credential-value','existing-engine-control','existing-engine-refresh',
        'odin-manager','odin-status'
    ]
    for id_ in required_ids:
        check(len(dom.ids.get(id_, [])) == 1, 'unique UI ID ' + id_)
    # These action buttons are outside HTML forms. An omitted type is
    # safe here; a submit type or a containing form would be unsafe.
    check(
        not re.search(r'<form\b', HTML.read_text(encoding='utf-8'), re.I),
        'action controls are outside HTML forms'
    )
    for id_ in ['rfpa-prepare','rfpa-enable','rfpa-confirm','rfpa-disable','rfpa-refresh']:
        button = dom.buttons.get(id_, {})
        check(
            bool(button) and button.get('type') in (None, 'button'),
            'safe action button ' + id_
        )
    scripts = ['app.js','config-editor.js','odin-ui.js','ui-rebuild.js',
               'engine-wizard.js','p39-compact.js','p53-ux.js','p54-workspace.js','p55-antiscan.js','svelte-status.js']
    check(dom.scripts == scripts, 'ordered frontend script imports')
    check('p53-ux.css' in HTML.read_text(encoding='utf-8'), 'P53 stylesheet linked')
    check('p54-workspace.css' in HTML.read_text(encoding='utf-8'), 'P54 stylesheet linked')
    check('p54-workspace.css' in BUILD.read_text(encoding='utf-8'), 'IPK build ships P54 stylesheet')
    check((FRONT / 'p53-ux.css').is_file(), 'P53 stylesheet exists')
    check('p53-ux.css' in BUILD.read_text(encoding='utf-8'), 'IPK build ships P53 stylesheet')
    for name in scripts:
        if name == 'svelte-status.js':
            continue  # Generated at package build time.
        check((FRONT / name).is_file(), 'script exists ' + name)
    build = BUILD.read_text(encoding='utf-8')
    for name in scripts:
        check(name in build, 'IPK build ships ' + name)
    svelte = (FRONT / 'PortAccessStatus.svelte').read_text(encoding='utf-8')
    legacy = (FRONT / 'app.js').read_text(encoding='utf-8')
    check('id="pa-svelte-status"' in HTML.read_text(encoding='utf-8'), 'native Svelte status mount')
    check('$state(' in svelte and 'onMount(' in svelte and 'refreshPreflight' in svelte and 'refreshStatus' in svelte, 'Svelte state owns status and preflight')
    check('renderStatus(' not in legacy and 'refreshPreflight(' not in legacy, 'legacy status controllers removed')
    check('npm run build' in build and 'build/svelte-status.js' in build, 'compiled Svelte IPK asset')
    js = (FRONT / 'ui-rebuild.js').read_text(encoding='utf-8')
    wiz = (FRONT / 'engine-wizard.js').read_text(encoding='utf-8')
    log = (FRONT / 'PortAccessOperationLog.svelte').read_text(encoding='utf-8')
    entry = (FRONT / 'status-main.js').read_text(encoding='utf-8')
    check(not (FRONT / 'operation-log.js').exists(), 'old log controller removed')
    check('id="pa-svelte-log"' in HTML.read_text(encoding='utf-8') and 'PortAccessOperationLog.svelte' in entry and 'mount(PortAccessOperationLog' in entry, 'Svelte operation log mount')
    check('$state(' in log and 'onMount(' in log and 'window.clearInterval(interval)' in log, 'native reactive live log with cleanup')
    check('data.scope' in log and "data.scope !== 'operations'" in log and 'Number.isSafeInteger(entry.id)' in log, 'read-only log API validation')
    check('operation-log.js' not in BUILD.read_text(encoding='utf-8').split('for file in ', 1)[1].split('; do', 1)[0], 'legacy log excluded from IPK copy list')
    check("var options=[{id:'odin'" in js and "{id:'knockd'" in js and "{id:'fwknopd'" in js, 'engine choice wiring')
    check("selector.value=id" in js and "selector.dispatchEvent" in js, 'engine selector synchronizes with wizard')
    check("engine.value === 'knockd' ?" in wiz, 'knock fields gated to knockd')
    check("lastValidatedRequest" in wiz, 'wizard validates before using draft')
    for item in ['rfpa-prepare','rfpa-enable','rfpa-confirm','rfpa-disable']:
        check(item in wiz, 'wizard handler for '+item)
    check('operation-logs' in log and 'window.setInterval(refresh, 2000)' in log, 'read-only live log polling')
    check('navigator.clipboard.writeText' in log and 'pa-log-pause' in log, 'live logs pause/copy controls')
    # Prevent accidentally introducing destructive UI workflows in a front-end-only acceptance patch.
    check('document.write(' not in js and 'eval(' not in js, 'no dynamic document writes or eval')
    if failures:
        print('P40A_UI_CONTRACT: FAIL (' + str(len(failures)) + ')')
        return 1
    print('P40A_UI_CONTRACT: PASS')
    return 0

if __name__ == '__main__':
    sys.exit(main())
